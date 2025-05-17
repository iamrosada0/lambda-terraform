package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Payload struct {
	Type string `json:"type"`
	Data struct {
		DeviceID  string  `json:"device_id"`
		Timestamp string  `json:"timestamp"`
		X         float64 `json:"x,omitempty"`
		Y         float64 `json:"y,omitempty"`
		Z         float64 `json:"z,omitempty"`
		Latitude  float64 `json:"latitude,omitempty"`
		Longitude float64 `json:"longitude,omitempty"`
		Image     string  `json:"image,omitempty"`
	} `json:"data"`
}

func main() {
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://localhost:1883").
		SetClientID("mqtt-to-sqs").
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("Error connecting to Mosquitto: %v\n", token.Error())
		os.Exit(1)
	}
	defer client.Disconnect(250)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(os.Getenv("AWS_DEFAULT_REGION")),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		fmt.Printf("Error loading AWS config: %v\n", err)
		os.Exit(1)
	}
	sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("LOCALSTACK_ENDPOINT"))
	})
}
