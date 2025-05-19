#!/bin/bash

# Variáveis de configuração
LOCALSTACK_ENDPOINT="http://localhost:4566"
SQS_QUEUE_NAME="minha-fila"
SQS_QUEUE_URL="${LOCALSTACK_ENDPOINT}/000000000000/${SQS_QUEUE_NAME}"
S3_TEST_BUCKET="my-test-bucket"
S3_LAMBDA_BUCKET="my-lambda-bucket"
DYNAMODB_TABLE="fleet-telemetry"
LAMBDA_FUNCTION_NAME="minha-funcao"
ZIP_FILE="go-lambda-localstack-test.zip"

# Verificar LocalStack
if ! curl -s "${LOCALSTACK_ENDPOINT}" > /dev/null; then
    echo "LocalStack não está rodando. Iniciando..."
    docker-compose down
    docker-compose up -d || { echo "Erro ao iniciar LocalStack"; exit 1; }
    sleep 10
fi

# Compilar
export GOOS=linux
export GOARCH=amd64
go build -tags lambda.norpc -o bootstrap main.go || { echo "Erro ao compilar"; exit 1; }

# Criar ZIP
if ! command -v build-lambda-zip &> /dev/null; then
    echo "build-lambda-zip não encontrado, usando zip..."
    zip "${ZIP_FILE}" bootstrap || { echo "Erro ao criar ZIP"; exit 1; }
else
    build-lambda-zip --output "${ZIP_FILE}" bootstrap || { echo "Erro ao criar ZIP"; exit 1; }
fi

# Upload para S3 (apenas se lambda.mountCode não for true)
if ! grep -q "mountCode: true" serverless.yml; then
    aws --endpoint-url="${LOCALSTACK_ENDPOINT}" s3 cp "${ZIP_FILE}" "s3://${S3_LAMBDA_BUCKET}/${ZIP_FILE}" || { echo "Erro ao fazer upload para S3"; exit 1; }
else
    echo "lambda.mountCode: true detectado, pulando upload para S3."
fi

# Aplicar Terraform
cd terraform
terraform init || { echo "Erro ao inicializar Terraform"; exit 1; }
terraform apply -auto-approve || { echo "Erro ao aplicar Terraform"; exit 1; }
cd ..

# Deploy Serverless
MSYS_NO_PATHCONV=1 serverless deploy --stage local || { echo "Erro ao implantar Serverless"; exit 1; }

# Testar
aws --endpoint-url="${LOCALSTACK_ENDPOINT}" sqs send-message \
    --queue-url "${SQS_QUEUE_URL}" \
    --message-body '{"type":"gps","data":{"device_id":"device123","timestamp":"2025-05-19T17:00:00Z","latitude":40.7128,"longitude":-74.0060}}' || { echo "Erro ao enviar mensagem SQS"; exit 1; }
serverless logs -f --stage local --function "${LAMBDA_FUNCTION_NAME}"
aws --endpoint-url="${LOCALSTACK_ENDPOINT}" dynamodb scan --table-name "${DYNAMODB_TABLE}"