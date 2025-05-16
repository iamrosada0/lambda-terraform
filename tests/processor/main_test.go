package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestHandleRequest_ValidGyroscope(t *testing.T) {
	payload := SQSPayload{
		Type: "gyroscope",
		Data: TelemetryData{
			DeviceID:  "mac123",
			Timestamp: "2025-05-16T12:00:00",
			X:         1.0,
			Y:         2.0,
			Z:         3.0,
		},
	}
	body, _ := json.Marshal(payload)
	event := events.SQSEvent{
		Records: []events.SQSMessage{{Body: string(body)}},
	}
	_, err := HandleRequest(context.Background(), event)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestHandleRequest_MissingFields(t *testing.T) {
	payload := SQSPayload{
		Type: "gps",
		Data: TelemetryData{DeviceID: "mac123"},
	}
	body, _ := json.Marshal(payload)
	event := events.SQSEvent{
		Records: []events.SQSMessage{{Body: string(body)}},
	}
	_, err := HandleRequest(context.Background(), event)
	if err != nil {
		t.Fatalf("Expected no error (skipped invalid data), got %v", err)
	}
}
