package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Environment constants with default values
const (
	defaultLocalstackEndpoint = "http://localhost:4566"
	defaultSQSQueueName       = "minha-fila"
	defaultSQSQueueURL        = defaultLocalstackEndpoint + "/000000000000/" + defaultSQSQueueName
	defaultS3TestBucket       = "my-test-bucket"
	defaultS3LambdaBucket     = "my-lambda-bucket"
	defaultDynamoDBTable      = "fleet-telemetry"
	defaultLambdaFunctionName = "minha-funcao"
	defaultZipFile            = "fleet-pulse.zip"
	defaultRegion             = "us-east-1"
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
	// Load environment variables with defaults
	localstackEndpoint := getEnv("LOCALSTACK_ENDPOINT", defaultLocalstackEndpoint)
	sqsQueueName := getEnv("SQS_QUEUE_NAME", defaultSQSQueueName)
	sqsQueueURL := getEnv("SQS_QUEUE_URL", defaultLocalstackEndpoint+"/000000000000/"+sqsQueueName)
	s3TestBucket := getEnv("S3_TEST_BUCKET", defaultS3TestBucket)
	s3LambdaBucket := getEnv("S3_LAMBDA_BUCKET", defaultS3LambdaBucket)
	dynamoDBTable := getEnv("DYNAMODB_TABLE", defaultDynamoDBTable)
	lambdaFunctionName := getEnv("LAMBDA_FUNCTION_NAME", defaultLambdaFunctionName)
	zipFile := getEnv("ZIP_FILE", defaultZipFile)
	region := getEnv("AWS_DEFAULT_REGION", defaultRegion)

	// Validate critical environment variables
	if localstackEndpoint == "" || sqsQueueURL == "" || region == "" {
		fmt.Println("Erro: LOCALSTACK_ENDPOINT, SQS_QUEUE_URL, ou AWS_DEFAULT_REGION não definidos")
		os.Exit(1)
	}
	fmt.Printf("Configuração:\n")
	fmt.Printf("  LOCALSTACK_ENDPOINT: %s\n", localstackEndpoint)
	fmt.Printf("  SQS_QUEUE_URL: %s\n", sqsQueueURL)
	fmt.Printf("  AWS_DEFAULT_REGION: %s\n", region)
	fmt.Printf("  SQS_QUEUE_NAME: %s\n", sqsQueueName)
	fmt.Printf("  S3_TEST_BUCKET: %s\n", s3TestBucket)
	fmt.Printf("  S3_LAMBDA_BUCKET: %s\n", s3LambdaBucket)
	fmt.Printf("  DYNAMODB_TABLE: %s\n", dynamoDBTable)
	fmt.Printf("  LAMBDA_FUNCTION_NAME: %s\n", lambdaFunctionName)
	fmt.Printf("  ZIP_FILE: %s\n", zipFile)

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
		o.BaseEndpoint = aws.String(localstackEndpoint)
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

		// Validate timestamp format
		if _, err := time.Parse(time.RFC3339, telemetry.Timestamp); err != nil {
			fmt.Printf("Erro: timestamp inválido: %s\n", telemetry.Timestamp)
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

		// Send to SQS with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    aws.String(sqsQueueURL),
			MessageBody: aws.String(string(body)),
		})
		if err != nil {
			fmt.Printf("Erro ao enviar para SQS: %v\n", err)
			return
		}
		fmt.Printf("Mensagem %s do dispositivo %s enviada para SQS com MessageId: %s\n", dataType, telemetry.DeviceID, *result.MessageId)
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

	// Manter o programa rodando com graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("Desconectando do Mosquitto...")
	client.Disconnect(250)
	fmt.Println("Programa encerrado")
}

// getEnv retrieves an environment variable or returns a default value if not set
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}
