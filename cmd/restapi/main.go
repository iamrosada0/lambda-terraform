package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

func HandleRequest(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	dataType := event.PathParameters["type"]
	if dataType != "gyroscope" && dataType != "gps" && dataType != "photo" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       `{"error": "Invalid data type. Use gyroscope, gps, or photo"}`,
		}, nil
	}

	var data TelemetryData
	if err := json.Unmarshal([]byte(event.Body), &data); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       `{"error": "Invalid JSON format"}`,
		}, nil
	}

	if data.DeviceID == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       `{"error": "Missing device_id"}`,
		}, nil
	}

	if data.Timestamp == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       `{"error": "Missing timestamp"}`,
		}, nil
	}

	switch dataType {
	case "gyroscope":
		if data.X == 0 && data.Y == 0 && data.Z == 0 {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       `{"error": "Missing or invalid gyroscope data (x, y, z)"}`,
			}, nil
		}
	case "gps":
		if data.Latitude == 0 && data.Longitude == 0 {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       `{"error": "Missing or invalid GPS data (latitude, longitude)"}`,
			}, nil
		}
	case "photo":
		if data.Image == "" {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       `{"error": "Missing photo data (image)"}`,
			}, nil
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-west-2"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: os.Getenv("LOCALSTACK_ENDPOINT")}, nil
			})),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf(`{"error": "Failed to load AWS config: %v"}`, err),
		}, nil
	}

	item, err := attributevalue.MarshalMap(data)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf(`{"error": "Failed to marshal data: %v"}`, err),
		}, nil
	}
	item["type"] = &types.AttributeValueMemberS{Value: dataType}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("DYNAMODB_TABLE")),
		Item:      item,
	})
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf(`{"error": "Failed to store data: %v"}`, err),
		}, nil
	}
}
