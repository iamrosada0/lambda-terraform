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

}
