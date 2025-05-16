terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "access_key" {
  type    = string
  default = "dummy"
}
variable "secret_key" {
  type    = string
  default = "dummy"

}
variable "region" {
  type    = string
  default = "us-west-2"
}
variable "localstack_endpoint" {
  type    = string
  default = "http://localhost:4566"

}
variable "sqs_queue_name" {
  type    = string
  default = "fleet-telemetry-queue"
}
variable "dynamodb_table_name" {
  type    = string
  default = "fleet-telemetry"

}

provider "aws" {
  access_key                  = var.access_key
  secret_key                  = var.secret_key
  region                      = var.region
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  endpoints {
    sqs         = var.localstack_endpoint
    lambda      = var.localstack_endpoint
    apigateway  = var.localstack_endpoint
    dynamodb    = var.localstack_endpoint
    rekognition = var.localstack_endpoint
    iam         = var.localstack_endpoint
  }
}

# SQS Queue
resource "aws_sqs_queue" "telemetry_queue" {
  name = var.sqs_queue_name
}

# DynamoDB Table
resource "aws_dynamodb_table" "telemetry_table" {
  name         = var.dynamodb_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "device_id"
  range_key    = "timestamp"

  attribute {
    name = "device_id"
    type = "S"
  }
  attribute {
    name = "timestamp"
    type = "S"
  }
}

# IAM Role for Lambda
resource "aws_iam_role" "lambda_role" {
  name = "lambda_execution_role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

# IAM Policy for Lambda
resource "aws_iam_role_policy" "lambda_policy" {
  name = "lambda_policy"
  role = aws_iam_role.lambda_role.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
        Resource = aws_sqs_queue.telemetry_queue.arn
      },
      {
        Effect   = "Allow"
        Action   = ["dynamodb:PutItem", "dynamodb:Query"]
        Resource = aws_dynamodb_table.telemetry_table.arn
      },
      {
        Effect   = "Allow"
        Action   = ["rekognition:CompareFaces"]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "*"
      }
    ]
  })
}

# Processor Lambda
resource "aws_lambda_function" "processor_lambda" {
  filename         = "processor_lambda.zip"
  function_name    = "telemetry_processor"
  role             = aws_iam_role.lambda_role.arn
  handler          = "main"
  runtime          = "go1.x"
  source_code_hash = filebase64sha256("processor_lambda.zip")
}

# REST API Lambda
resource "aws_lambda_function" "restapi_lambda" {
  filename         = "restapi_lambda.zip"
  function_name    = "telemetry_restapi"
  role             = aws_iam_role.lambda_role.arn
  handler          = "main"
  runtime          = "go1.x"
  source_code_hash = filebase64sha256("restapi_lambda.zip")
}

# API Gateway
resource "aws_api_gateway_rest_api" "telemetry_api" {
  name = "telemetry-api"
}

resource "aws_api_gateway_resource" "telemetry" {
  rest_api_id = aws_api_gateway_rest_api.telemetry_api.id
  parent_id   = aws_api_gateway_rest_api.telemetry_api.root_resource_id
  path_part   = "telemetry"
}

resource "aws_api_gateway_resource" "telemetry_type" {
  rest_api_id = aws_api_gateway_rest_api.telemetry_api.id
  parent_id   = aws_api_gateway_resource.telemetry.id
  path_part   = "{type}"
}

resource "aws_api_gateway_method" "post_method" {
  rest_api_id   = aws_api_gateway_rest_api.telemetry_api.id
  resource_id   = aws_api_gateway_resource.telemetry_type.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "lambda_integration" {
  rest_api_id             = aws_api_gateway_rest_api.telemetry_api.id
  resource_id             = aws_api_gateway_resource.telemetry_type.id
  http_method             = aws_api_gateway_method.post_method.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.restapi_lambda.invoke_arn
}

resource "aws_api_gateway_deployment" "deployment" {
  rest_api_id = aws_api_gateway_rest_api.telemetry_api.id
  depends_on  = [aws_api_gateway_integration.lambda_integration]
  stage_name  = "prod"
}

resource "aws_lambda_permission" "api_gateway_permission" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.restapi_lambda.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.telemetry_api.execution_arn}/*/*"
}

# SQS to Processor Lambda Mapping
resource "aws_lambda_event_source_mapping" "sqs_trigger" {
  event_source_arn = aws_sqs_queue.telemetry_queue.arn
  function_name    = aws_lambda_function.processor_lambda.arn
}
