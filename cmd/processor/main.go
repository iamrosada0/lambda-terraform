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

}
