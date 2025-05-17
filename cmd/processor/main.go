package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	rekoTypes "github.com/aws/aws-sdk-go-v2/service/rekognition/types" // Correct import for Image
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// TelemetryData represents the structure of incoming telemetry data
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

// SQSPayload represents the SQS message structure
type SQSPayload struct {
	Type string        `json:"type"`
	Data TelemetryData `json:"data"`
}

// HandleRequest processes SQS messages containing telemetry data
func HandleRequest(ctx context.Context, event events.SQSEvent) (string, error) {
	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(os.Getenv("AWS_DEFAULT_REGION")),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Service clients with LocalStack endpoints
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

	// Process each SQS message
	for _, record := range event.Records {
		fmt.Printf("Recebendo mensagem ID: %s\n", record.MessageId)
		fmt.Printf("Conteúdo da mensagem: %s\n", record.Body)
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
				TargetImage:         &rekoTypes.Image{Bytes: imageBytes}, // Placeholder: replace with stored photo
				SimilarityThreshold: aws.Float32(70),
			})
			isRecognized := err == nil && len(resp.FaceMatches) > 0
			item["is_recognized"], _ = attributevalue.Marshal(isRecognized)
			item["s3_key"], _ = attributevalue.Marshal(key) // Store S3 key in DynamoDB
		default:
			fmt.Println("Unknown data type")
			continue
		}

		// Store in DynamoDB
		_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(os.Getenv("DYNAMODB_TABLE")),
			Item:      item,
		})
		if err != nil {
			fmt.Printf("Error storing %s data: %v\n", dataType, err)
			continue
		}

		// Delete SQS message
		_, err = sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(os.Getenv("SQS_QUEUE_URL")),
			ReceiptHandle: aws.String(record.ReceiptHandle),
		})
		if err != nil {
			fmt.Printf("Error deleting SQS message: %v\n", err)
		}
	}

	return "Processed successfully", nil
}

func main() {
	lambda.Start(HandleRequest)
}
