package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// TelemetryData represents the structure of incoming telemetry data
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

// SQSPayload represents the SQS message structure
type SQSPayload struct {
	Type string        `json:"type"`
	Data TelemetryData `json:"data"`
}

func main() {
	// Validate environment variables
	endpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	queueURL := os.Getenv("SQS_QUEUE_URL")
	region := os.Getenv("AWS_DEFAULT_REGION")
	if endpoint == "" || queueURL == "" || region == "" {
		fmt.Println("Erro: LOCALSTACK_ENDPOINT, SQS_QUEUE_URL, ou AWS_DEFAULT_REGION não definidos")
		os.Exit(1)
	}
	fmt.Printf("Usando LOCALSTACK_ENDPOINT: %s\n", endpoint)
	fmt.Printf("Usando SQS_QUEUE_URL: %s\n", queueURL)
	fmt.Printf("Usando AWS_DEFAULT_REGION: %s\n", region)

	// Configurar cliente SQS
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		fmt.Printf("Erro ao carregar configuração AWS: %v\n", err)
		os.Exit(1)
	}
	sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	fmt.Println("Cliente SQS configurado com sucesso")

	// Configurar cliente MQTT
	messageHandler := func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Recebida mensagem no tópico %s: %s\n", msg.Topic(), string(msg.Payload()))
		var telemetry TelemetryData
		if err := json.Unmarshal(msg.Payload(), &telemetry); err != nil {
			fmt.Printf("Erro ao deserializar mensagem MQTT: %v\n", err)
			return
		}

		// Validate required fields
		if telemetry.DeviceID == "" || telemetry.Timestamp == "" {
			fmt.Println("Erro: device_id ou timestamp ausentes")
			return
		}

		// Validate type-specific fields
		dataType := strings.TrimPrefix(msg.Topic(), "sensor/")
		switch dataType {
		case "gyroscope":
			if telemetry.X == 0 && telemetry.Y == 0 && telemetry.Z == 0 {
				fmt.Println("Erro: dados de giroscópio inválidos")
				return
			}
		case "gps":
			if telemetry.Latitude == 0 && telemetry.Longitude == 0 {
				fmt.Println("Erro: dados de GPS inválidos")
				return
			}
		case "photo":
			if telemetry.Image == "" {
				fmt.Println("Erro: dados de foto inválidos")
				return
			}
		default:
			fmt.Printf("Erro: tipo de dados desconhecido: %s\n", dataType)
			return
		}

		// Construct SQS payload
		payload := SQSPayload{
			Type: dataType,
			Data: telemetry,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("Erro ao serializar payload para SQS: %v\n", err)
			return
		}
		fmt.Printf("Enviando para SQS: %s\n", string(body))
		result, err := sqsClient.SendMessage(context.Background(), &sqs.SendMessageInput{
			QueueUrl:    aws.String(queueURL),
			MessageBody: aws.String(string(body)),
		})
		if err != nil {
			fmt.Printf("Erro ao enviar para SQS: %v\n", err)
			return
		}
		fmt.Printf("Mensagem %s enviada para SQS com MessageId: %s\n", dataType, *result.MessageId)
	}

	opts := mqtt.NewClientOptions().
		AddBroker("tcp://localhost:1883").
		SetClientID("mqtt-to-sqs").
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetConnectionLostHandler(func(client mqtt.Client, err error) {
			fmt.Printf("Conexão MQTT perdida: %v\n", err)
		}).
		SetOnConnectHandler(func(client mqtt.Client) {
			fmt.Println("Conexão MQTT estabelecida com sucesso")
			// Reinscrever no tópico ao reconectar
			if token := client.Subscribe("sensor/#", 0, messageHandler); token.Wait() && token.Error() != nil {
				fmt.Printf("Erro ao reinscrever no tópico sensor/#: %v\n", token.Error())
				os.Exit(1)
			}
			fmt.Println("Re-inscrito no tópico sensor/# com sucesso")
		}).
		SetProtocolVersion(4) // MQTT 3.1.1

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("Erro ao conectar ao Mosquitto: %v\n", token.Error())
		os.Exit(1)
	}
	fmt.Println("Conectado ao Mosquitto com sucesso")

	// Inscrever-se no tópico MQTT
	if token := client.Subscribe("sensor/#", 0, messageHandler); token.Wait() && token.Error() != nil {
		fmt.Printf("Erro ao inscrever no tópico sensor/#: %v\n", token.Error())
		os.Exit(1)
	}
	fmt.Println("Inscrito no tópico sensor/# com sucesso")

	// Manter o programa rodando
	select {}
}
