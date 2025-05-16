package main

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
