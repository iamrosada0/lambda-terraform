package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type TelemetryData struct {
	DeviceID  string  `json:"device_id"`
	Timestamp string  `json:"timestamp"`
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	Z         float64 `json:"z,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Image     string  `json:"image,omitempty"`
}

type SQSPayload struct {
	Type string        `json:"type"`
	Data TelemetryData `json:"data"`
}

func HandleRequest(ctx context.Context, event events.SQSEvent) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-west-2"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: os.Getenv("LOCALSTACK_ENDPOINT")}, nil
			})),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %v", err)
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)
	rekognitionClient := rekognition.NewFromConfig(cfg)
for _, record := range event.Records {
	var payload SQSPayload
	if err := json.Unmarshal([]byte(record.Body), &payload); err != nil {
		fmt.Printf("Error unmarshaling SQS message: %v\n", err)
		continue
	}

	data := payload.Data
	dataType := payload.Type
	if data.DeviceID == "" || data.Timestamp == "" {
		fmt.Println("Missing device_id or timestamp")
		continue
	}
item := map[string]types.AttributeValue{
	"device_id": &types.AttributeValueMemberS{Value: data.DeviceID},
	"timestamp": &types.AttributeValueMemberS{Value: data.Timestamp},
	"type":      &types.AttributeValueMemberS{Value: dataType},
}

switch dataType {
case "gyroscope":
	if data.X == 0 && data.Y == 0 && data.Z == 0 {
		fmt.Println("Invalid gyroscope data")
		continue
	}
	item["x"], _ = attributevalue.Marshal(data.X)
	item["y"], _ = attributevalue.Marshal(data.Y)
	item["z"], _ = attributevalue.Marshal(data.Z)
case "gps":
	if data.Latitude == 0 && data.Longitude == 0 {
		fmt.Println("Invalid GPS data")
		continue
	}
	item["latitude"], _ = attributevalue.Marshal(data.Latitude)
	item["longitude"], _ = attributevalue.Marshal(data.Longitude)
case "photo":
	if data.Image == "" {
		fmt.Println("Invalid photo data")
		continue
	}
	item["image"], _ = attributevalue.Marshal(data.Image)
	imageBytes, err := base64.StdEncoding.DecodeString(data.Image)
	if err != nil {
		fmt.Printf("Error decoding photo: %v\n", err)
		continue
	}
	// Placeholder Rekognition call
	resp, err := rekognitionClient.CompareFaces(ctx, &rekognition.CompareFacesInput{
		SourceImage:         &types.Image{Bytes: imageBytes},
		TargetImage:         &types.Image{Bytes: imageBytes}, // Placeholder
		SimilarityThreshold: aws.Float32(70),
	})
	isRecognized := err == nil && len(resp.FaceMatches) > 0
	item["is_recognized"], _ = attributevalue.Marshal(isRecognized)
default:
	fmt.Println("Unknown data type")
	continue
}
_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
	TableName: aws.String(os.Getenv("DYNAMODB_TABLE")),
	Item:      item,
})
if err != nil {
	fmt.Printf("Error storing %s data: %v\n", dataType, err)
	continue
}
_, err = sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
	QueueUrl:      aws.String(os.Getenv("SQS_QUEUE_URL")),
	ReceiptHandle: aws.String(record.ReceiptHandle),
})
if err != nil {
	fmt.Printf("Error deleting SQS message: %v\n", err)
}

}
