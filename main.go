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
	rekoTypes "github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSPayload defines the structure of the SQS message payload
type SQSPayload struct {
	Type string `json:"type"`
	Data Data   `json:"data"`
}

// Data defines the telemetry data structure
type Data struct {
	DeviceID  string  `json:"device_id"`
	Timestamp string  `json:"timestamp"`
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	Z         float64 `json:"z,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Image     string  `json:"image,omitempty"`
}

// HandleRequest processes SQS messages containing telemetry data
func HandleRequest(ctx context.Context, event events.SQSEvent) (string, error) {
	// Load env vars with defaults
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = "us-east-1" // Ajustado para us-east-1, consistente com o projeto
	}

	localstackEndpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	if localstackEndpoint == "" {
		localstackEndpoint = "http://host.docker.internal:4566" // Ajustado para Windows
	}

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		s3Bucket = "my-test-bucket"
	}

	dynamoTable := os.Getenv("DYNAMODB_TABLE")
	if dynamoTable == "" {
		dynamoTable = "fleet-telemetry"
	}

	sqsQueueURL := os.Getenv("SQS_QUEUE_URL")
	if sqsQueueURL == "" {
		sqsQueueURL = "http://localhost:4566/000000000000/minha-fila" // Ajustado para LocalStack
	}

	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Service clients with LocalStack endpoints
	dynamoClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(localstackEndpoint)
	})
	sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(localstackEndpoint)
	})
	rekClient := rekognition.NewFromConfig(cfg, func(o *rekognition.Options) {
		o.BaseEndpoint = aws.String(localstackEndpoint)
	})
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(localstackEndpoint)
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
			key := fmt.Sprintf("%s/%s.jpg", data.DeviceID, data.Timestamp)
			_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(s3Bucket),
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
				TargetImage:         &rekoTypes.Image{Bytes: imageBytes}, // Placeholder
				SimilarityThreshold: aws.Float32(70),
			})
			isRecognized := err == nil && len(resp.FaceMatches) > 0
			item["is_recognized"], _ = attributevalue.Marshal(isRecognized)
			item["s3_key"], _ = attributevalue.Marshal(key)
		default:
			fmt.Println("Unknown data type")
			continue
		}

		// Store in DynamoDB
		_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(dynamoTable),
			Item:      item,
		})
		if err != nil {
			fmt.Printf("Error storing %s data: %v\n", dataType, err)
			continue
		}

		// Delete SQS message
		_, err = sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(sqsQueueURL),
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
