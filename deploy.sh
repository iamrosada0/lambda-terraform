#!/bin/bash
set -e

# Set environment variables for AWS CLI and LocalStack
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy
export AWS_DEFAULT_REGION=us-east-2
export LOCALSTACK_ENDPOINT=http://localhost:4566
# Endpoints do LocalStack (region-aware e legacy)
export S3_LOCALSTACK_ENDPOINT=http://s3.localhost.localstack.cloud:4566
# Recursos (nomes de bucket, fila e tabela DynamoDB)
export S3_BUCKET=my-test-bucket
export SQS_QUEUE=minha-fila
export DYNAMODB_TABLE=fleet-telemetry
# URL completa da fila SQS para facilitar o uso no código
export SQS_QUEUE_URL=http://sqs.${AWS_DEFAULT_REGION}.localhost.localstack.cloud:4566/000000000000/${SQS_QUEUE}
export LAMBDA_RUNTIME_ENVIRONMENT_TIMEOUT=30

# Lambda function names
PROCESSOR_LAMBDA_NAME="telemetry_processor"
RESTAPI_LAMBDA_NAME="telemetry_restapi"

# Function to check if a Docker image exists
check_docker_image() {
  local IMAGE_NAME=$1
  if ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
    echo "Docker image '$IMAGE_NAME' is missing. Please pull it."
    return 1 # Return a non-zero exit code to indicate failure
  fi
  return 0
}

echo "1. Build and zip the functions using docker-compose..."
# Build the functions.
docker-compose run --rm builder-processor
mv cmd/processor/function.zip terraform/localstack/processor.zip
if [ ! -f terraform/localstack/processor.zip ]; then
  echo "Error: terraform/localstack/processor.zip not found. Build failed."
  exit 1
fi

docker-compose run --rm builder-restapi
mv cmd/restapi/function.zip terraform/localstack/restapi.zip
if [ ! -f terraform/localstack/restapi.zip ]; then
  echo "Error: terraform/localstack/restapi.zip not found. Build failed."
  exit 1
fi

echo "2. Starting LocalStack..."
# Start LocalStack in detached mode
docker-compose up -d localstack

echo "Waiting for LocalStack to start..."
# Wait for LocalStack to be ready
until aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION sqs list-queues >/dev/null 2>&1; do
  echo -n "."
  sleep 2
done
echo "  LocalStack is running."

echo "3. Applying Terraform configuration..."
# Apply Terraform
cd terraform/localstack
terraform init -input=false
terraform apply -auto-approve
if [ $? -ne 0 ]; then
  echo "Terraform apply failed. Exiting..."
  exit 1
fi
cd ../..

# Function to check if a Lambda function exists
function check_lambda() {
  local FUNC_NAME=$1

  echo "Checking if Lambda function '$FUNC_NAME' exists..."
  if aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda get-function --function-name $FUNC_NAME >/dev/null 2>&1; then
    echo "Lambda function '$FUNC_NAME' exists."
  else
    echo "Lambda function '$FUNC_NAME' DOES NOT exist."
  fi
}

# Function to invoke a Lambda function and check the response.
function invoke_lambda() {
  local FUNC_NAME=$1
  local PAYLOAD='{}' # You can adjust the payload as needed.
  local TEMP_FILE=$(mktemp)

  echo "Invoking Lambda function '$FUNC_NAME' with payload '$PAYLOAD'..."
  if aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda invoke \
         --function-name "$FUNC_NAME" --payload "$PAYLOAD" "$TEMP_FILE" >/dev/null 2>&1; then
    RESPONSE=$(cat "$TEMP_FILE")
    if echo "$RESPONSE" | grep -q '"StatusCode":200'; then
      echo "Lambda function '$FUNC_NAME' invoked successfully."
      echo "Response: $RESPONSE"
    else
      echo "Lambda function '$FUNC_NAME' invocation failed."
      echo "Response: $RESPONSE"
      echo "Check LocalStack logs for details (docker logs <localstack-container>)."
      rm -f "$TEMP_FILE"
      return 1
    fi
  else
    echo "Error invoking Lambda function '$FUNC_NAME'."
    RESPONSE=$(cat "$TEMP_FILE" 2>/dev/null || echo "No response available")
    echo "Response: $RESPONSE"
    echo "Check LocalStack logs for details (docker logs <localstack-container>)."
    rm -f "$TEMP_FILE"
    return 1
  fi
  rm -f "$TEMP_FILE"
  return 0
}

# Check and remove the actual Lambda functions if they exist
echo "4. Verificando se as funções Lambda existem..."
for FUNC_NAME in $PROCESSOR_LAMBDA_NAME $RESTAPI_LAMBDA_NAME; do
  if aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda get-function --function-name $FUNC_NAME >/dev/null 2>&1; then
    echo "Função Lambda '$FUNC_NAME' já existe. Removendo..."
    aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda delete-function --function-name $FUNC_NAME
  else
    echo "Função Lambda '$FUNC_NAME' não existe."
  fi
done

# Check if IAM role exists or create it
echo "5. Verificando/criando IAM Role para Lambda..."
if aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION iam get-role --role-name lambda_role >/dev/null 2>&1; then
  echo "Role 'lambda_role' já existe. Reutilizando..."
  ROLE_ARN=$(aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION iam get-role \
    --role-name lambda_role \
    --query 'Role.Arn' --output text)
else
  echo "Criando role 'lambda_role'..."
  ROLE_ARN=$(aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION iam create-role \
    --role-name lambda_role \
    --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Action":"sts:AssumeRole","Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"}}]}' \
    --query 'Role.Arn' --output text)
fi

# Attach or update IAM policy to the role
echo "6. Anexando/atualizando política IAM à Role..."
aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION iam put-role-policy \
  --role-name lambda_role \
  --policy-name lambda_policy \
  --policy-document '{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes",
          "sqs:GetQueueUrl"
        ],
        "Resource": "arn:aws:sqs:us-east-2:000000000000:minha-fila"
      },
      {
        "Effect": "Allow",
        "Action": [
          "dynamodb:PutItem",
          "dynamodb:Query",
          "dynamodb:GetItem",
          "dynamodb:UpdateItem",
          "dynamodb:Scan"
        ],
        "Resource": "arn:aws:dynamodb:us-east-2:000000000000:table/fleet-telemetry"
      },
      {
        "Effect": "Allow",
        "Action": [
          "s3:PutObject",
          "s3:GetObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ],
        "Resource": [
          "arn:aws:s3:::my-test-bucket",
          "arn:aws:s3:::my-test-bucket/*"
        ]
      },
      {
        "Effect": "Allow",
        "Action": ["rekognition:CompareFaces"],
        "Resource": "*"
      },
      {
        "Effect": "Allow",
        "Action": [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ],
        "Resource": "*"
      }
    ]
  }'

echo "7. Criando funções Lambda 'telemetry_processor' e 'telemetry_restapi'..."
# Create telemetry_processor Lambda
if ! aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda create-function \
  --function-name $PROCESSOR_LAMBDA_NAME \
  --runtime provided.al2 \
  --role $ROLE_ARN \
  --handler bootstrap \
  --zip-file fileb://terraform/localstack/processor.zip \
  --architectures "x86_64" \
  --environment "Variables={MQTT_BROKER=mosquitto:1883,S3_BUCKET=$S3_BUCKET,DYNAMODB_TABLE=$DYNAMODB_TABLE,SQS_QUEUE_URL=$SQS_QUEUE_URL,AWS_DEFAULT_REGION=$AWS_DEFAULT_REGION}" > /dev/null 2>&1; then
  echo "Error creating Lambda function '$PROCESSOR_LAMBDA_NAME'. Check ZIP file or LocalStack logs."
  exit 1
fi
echo "Lambda function '$PROCESSOR_LAMBDA_NAME' created successfully."

# Create telemetry_restapi Lambda
if ! aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda create-function \
  --function-name $RESTAPI_LAMBDA_NAME \
  --runtime provided.al2 \
  --role $ROLE_ARN \
  --handler bootstrap \
  --zip-file fileb://terraform/localstack/restapi.zip \
  --architectures "x86_64" \
  --environment "Variables={MQTT_BROKER=mosquitto:1883,S3_BUCKET=$S3_BUCKET,DYNAMODB_TABLE=$DYNAMODB_TABLE,SQS_QUEUE_URL=$SQS_QUEUE_URL,AWS_DEFAULT_REGION=$AWS_DEFAULT_REGION}" > /dev/null 2>&1; then
  echo "Error creating Lambda function '$RESTAPI_LAMBDA_NAME'. Check ZIP file or LocalStack logs."
  exit 1
fi
echo "Lambda function '$RESTAPI_LAMBDA_NAME' created successfully."

echo "8. Verificando mapeamentos de evento para as funções Lambda..."
QUEUE_ARN=$(aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION sqs get-queue-attributes \
  --queue-url $SQS_QUEUE_URL \
  --attribute-names QueueArn \
  --query 'Attributes.QueueArn' \
  --output text)

for FUNC_NAME in $PROCESSOR_LAMBDA_NAME; do
  MAPPING_UUID=$(aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda list-event-source-mappings \
    --function-name $FUNC_NAME \
    --query "EventSourceMappings[?EventSourceArn=='$QUEUE_ARN'].UUID" \
    --output text)
  if [ -n "$MAPPING_UUID" ]; then
    echo "Mapeamento de evento já existe para '$FUNC_NAME' (UUID: $MAPPING_UUID). Removendo..."
    aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda delete-event-source-mapping --uuid $MAPPING_UUID
  else
    echo "Nenhum mapeamento de evento encontrado para '$FUNC_NAME' na fila SQS."
  fi
done

echo "9. Configurando mapeamento e permissões..."
# Create event source mapping for telemetry_processor
aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda create-event-source-mapping \
  --function-name $PROCESSOR_LAMBDA_NAME \
  --batch-size 1 \
  --event-source-arn $QUEUE_ARN

# Add API Gateway permission for telemetry_restapi
echo "Adicionando permissão para API Gateway invocar 'telemetry_restapi'..."
aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda add-permission \
  --function-name $RESTAPI_LAMBDA_NAME \
  --statement-id AllowAPIGatewayInvoke \
  --action lambda:InvokeFunction \
  --principal apigateway.amazonaws.com \
  --source-arn "arn:aws:execute-api:us-east-2:000000000000:*/prod/*/*"

# Check the lambdas.
check_lambda $PROCESSOR_LAMBDA_NAME
check_lambda $RESTAPI_LAMBDA_NAME

# Invoke the lambdas.
echo "10. Invoking Lambda functions..."
invoke_lambda $PROCESSOR_LAMBDA_NAME
invoke_lambda $RESTAPI_LAMBDA_NAME

echo "✅ Verification complete!"

echo "11. Environment setup complete. LocalStack remains running for further testing."
echo "  Use 'docker-compose down' manually to tear down the environment."