#!/bin/bash
set -e

export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy
export AWS_DEFAULT_REGION=us-west-2
export LOCALSTACK_ENDPOINT=http://localhost:4566
export SQS_QUEUE=minha-fila

PROCESSOR_LAMBDA_NAME="processor-func"
RESTAPI_LAMBDA_NAME="restapi-func"

echo "1. Build e zip das funções via docker-compose..."

docker-compose run --rm builder-processor
mv cmd/processor/function.zip terraform/localstack/processor.zip

docker-compose run --rm builder-restapi
mv cmd/restapi/function.zip terraform/localstack/restapi.zip

echo "2. Subindo LocalStack..."
docker-compose up -d localstack

echo "Aguardando LocalStack iniciar..."
until aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION sqs list-queues >/dev/null 2>&1; do
  echo -n "."
  sleep 2
done
echo " LocalStack está rodando."

# (o resto igual ao script anterior, chamando deploy_lambda para processor.zip e restapi.zip)

function deploy_lambda() {
  local FUNC_NAME=$1
  local ZIP_FILE=$2

  echo "Verificando se a função Lambda '$FUNC_NAME' existe..."
  if aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda get-function --function-name $FUNC_NAME >/dev/null 2>&1; then
    echo "Função Lambda '$FUNC_NAME' já existe. Removendo..."
    aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda delete-function --function-name $FUNC_NAME
  else
    echo "Função Lambda '$FUNC_NAME' não existe."
  fi

  echo "Criando função Lambda '$FUNC_NAME'..."
  aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda create-function \
    --function-name $FUNC_NAME \
    --runtime provided.al2 \
    --role arn:aws:iam::000000000000:role/lambda-execute \
    --handler bootstrap \
    --zip-file fileb://$ZIP_FILE \
    --architectures "x86_64"

  echo "Verificando mapeamento de evento para '$FUNC_NAME'..."
  QUEUE_URL=http://localhost:4566/000000000000/$SQS_QUEUE
  QUEUE_ARN=$(aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION sqs get-queue-attributes \
    --queue-url $QUEUE_URL \
    --attribute-names QueueArn \
    --query 'Attributes.QueueArn' \
    --output text)

  MAPPING_UUID=$(aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda list-event-source-mappings \
    --function-name $FUNC_NAME \
    --query "EventSourceMappings[?EventSourceArn=='$QUEUE_ARN'].UUID" \
    --output text)

  if [ -n "$MAPPING_UUID" ]; then
    echo "Mapeamento de evento já existe (UUID: $MAPPING_UUID). Removendo..."
    aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda delete-event-source-mapping --uuid $MAPPING_UUID
  else
    echo "Nenhum mapeamento de evento encontrado para a fila SQS."
  fi

  echo "Criando event source mapping para ligar SQS à Lambda '$FUNC_NAME'..."
  aws --endpoint-url=$LOCALSTACK_ENDPOINT --region $AWS_DEFAULT_REGION lambda create-event-source-mapping \
    --function-name $FUNC_NAME \
    --batch-size 1 \
    --event-source-arn $QUEUE_ARN

  echo "Função '$FUNC_NAME' deployada com sucesso!"
}

deploy_lambda $PROCESSOR_LAMBDA_NAME processor.zip
deploy_lambda $RESTAPI_LAMBDA_NAME restapi.zip

echo "✅ Deploy finalizado!"
