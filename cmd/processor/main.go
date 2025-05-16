package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
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

}
