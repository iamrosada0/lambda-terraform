package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type TelemetryData struct {
	DeviceID  string  `json:"device_id"` // MAC address of the device
	Timestamp string  `json:"timestamp"` // ISO 8601 timestamp
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	Z         float64 `json:"z,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Image     string  `json:"image,omitempty"` // Base64-encoded photo
}
type SQSPayload struct {
	Type string        `json:"type"`
	Data TelemetryData `json:"data"`
}

func HandleRequest(ctx context.Context, event events.SQSEvent) (string, error) {

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-west-2"),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("LOCALSTACK_ENDPOINT"))
	})
	sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("LOCALSTACK_ENDPOINT"))
	})
	rekClient := rekognition.NewFromConfig(cfg, func(o *rekognition.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("LOCALSTACK_ENDPOINT"))
	})
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("LOCALSTACK_ENDPOINT"))
		o.UsePathStyle = true
	})


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

		// Prepare DynamoDB item
		item := map[string]types.AttributeValue{
			"device_id": &types.AttributeValueMemberS{Value: data.DeviceID},
			"timestamp": &types.AttributeValueMemberS{Value: data.Timestamp},
			"type":      &types.AttributeValueMemberS{Value: dataType},
		}

		// Validate and process data based on type
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

			// Decode and upload photo to S3
			imageBytes, err := base64.StdEncoding.DecodeString(data.Image)
			if err != nil {
				fmt.Printf("Error decoding photo: %v\n", err)
				continue
			}
			bucketName := os.Getenv("S3_BUCKET")
			key := fmt.Sprintf("%s/%s.jpg", data.DeviceID, data.Timestamp)
			_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(key),
				Body:   bytes.NewReader(imageBytes),
			})
			if err != nil {
				fmt.Printf("Error uploading photo to S3: %v\n", err)
				continue
			}

			// Placeholder Rekognition call
			resp, err := rekClient.CompareFaces(ctx, &rekognition.CompareFacesInput{
				SourceImage:         &rekoTypes.Image{Bytes: imageBytes}, 
				TargetImage:         &rekoTypes.Image{Bytes: imageBytes}, 
				SimilarityThreshold: aws.Float32(70),
			})
			isRecognized := err == nil && len(resp.FaceMatches) > 0
			item["is_recognized"], _ = attributevalue.Marshal(isRecognized)
			item["s3_key"], _ = attributevalue.Marshal(key) // Store S3 key in DynamoDB
		default:
			fmt.Println("Unknown data type")
			continue
		}


}
