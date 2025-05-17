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

type Payload struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func main() {
	// Configurar cliente MQTT
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
			if token := client.Subscribe("sensor/#", 0, nil); token.Wait() && token.Error() != nil {
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

	// Configurar cliente SQS
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(os.Getenv("AWS_DEFAULT_REGION")),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		fmt.Printf("Erro ao carregar configuração AWS: %v\n", err)
		os.Exit(1)
	}
	endpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	queueURL := os.Getenv("SQS_QUEUE_URL")
	fmt.Printf("Usando LOCALSTACK_ENDPOINT: %s\n", endpoint)
	fmt.Printf("Usando SQS_QUEUE_URL: %s\n", queueURL)
	sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	fmt.Println("Cliente SQS configurado com sucesso")

	// Inscrever-se no tópico MQTT
	if token := client.Subscribe("sensor/#", 0, func(c mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Recebida mensagem no tópico %s: %s\n", msg.Topic(), string(msg.Payload()))
		var data Payload
		if err := json.Unmarshal(msg.Payload(), &data.Data); err != nil {
			fmt.Printf("Erro ao deserializar mensagem MQTT: %v\n", err)
			return
		}
		data.Type = strings.TrimPrefix(msg.Topic(), "sensor/")
		body, err := json.Marshal(data)
		if err != nil {
			fmt.Printf("Erro ao serializar payload para SQS: %v\n", err)
			return
		}
		fmt.Printf("Enviando para SQS: %s\n", string(body))
		_, err = sqsClient.SendMessage(context.Background(), &sqs.SendMessageInput{
			QueueUrl:    aws.String(queueURL),
			MessageBody: aws.String(string(body)),
		})
		if err != nil {
			fmt.Printf("Erro ao enviar para SQS: %v\n", err)
			return
		}
		fmt.Printf("Enviado %s para SQS\n", data.Type)
	}); token.Wait() && token.Error() != nil {
		fmt.Printf("Erro ao inscrever no tópico sensor/#: %v\n", token.Error())
		os.Exit(1)
	}
	fmt.Println("Inscrito no tópico sensor/# com sucesso")

	// Manter o programa rodando
	select {}
}
