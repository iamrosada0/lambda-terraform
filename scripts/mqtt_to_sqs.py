import paho.mqtt.client as mqtt
import boto3
import json
import os

MQTT_BROKER = "localhost"
MQTT_PORT = 1883
MQTT_TOPICS = ["sensor/gyroscope", "sensor/gps", "sensor/photo"]
SQS_QUEUE_URL = os.getenv("SQS_QUEUE_URL")
LOCALSTACK_ENDPOINT = os.getenv("LOCALSTACK_ENDPOINT")

sqs = boto3.client(
    "sqs",
    endpoint_url=LOCALSTACK_ENDPOINT,
    region_name="us-west-2",
    aws_access_key_id="dummy",
    aws_secret_access_key="dummy"
)

def on_connect(client, userdata, flags, rc):
    print(f"Connected to MQTT broker with code {rc}")
    for topic in MQTT_TOPICS:
        client.subscribe(topic)

def on_message(client, userdata, msg):
    payload = msg.payload.decode()
    topic = msg.topic
    print(f"Received MQTT message on {topic}: {payload}")
    try:
        sqs.send_message(
            QueueUrl=SQS_QUEUE_URL,
            MessageBody=json.dumps({"type": topic.split("/")[-1], "data": json.loads(payload)})
        )
        print(f"Sent to SQS")
    except Exception as e:
        print(f"Error sending to SQS: {e}")

client = mqtt.Client()
client.on_connect = on_connect
client.on_message = on_message
client.connect(MQTT_BROKER, MQTT_PORT, 60)
client.loop_start()

try:
    while True:
        pass
except KeyboardInterrupt:
    client.loop_stop()
    client.disconnect()