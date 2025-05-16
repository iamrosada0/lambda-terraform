package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestHandleRequest_ValidGyroscope(t *testing.T) {
	event := events.APIGatewayProxyRequest{
		PathParameters: map[string]string{"type": "gyroscope"},
		Body:           `{"device_id": "mac123", "timestamp": "2025-05-16T12:00:00", "x": 1.0, "y": 2.0, "z": 3.0}`,
	}
	resp, err := HandleRequest(context.Background(), event)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.Unmarshal([]byte(resp.Body), &body)
	if body["message"] != "gyroscope data stored successfully" {
		t.Errorf("Expected message, got %v", body["message"])
	}
}

func TestHandleRequest_MissingFields(t *testing.T) {
	event := events.APIGatewayProxyRequest{
		PathParameters: map[string]string{"type": "gps"},
		Body:           `{"device_id": "mac123"}`,
	}
	resp, err := HandleRequest(context.Background(), event)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.Unmarshal([]byte(resp.Body), &body)
	if body["error"] != "Missing timestamp" {
		t.Errorf("Expected missing timestamp error, got %v", body["error"])
	}
}
