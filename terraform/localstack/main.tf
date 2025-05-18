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
  default = "us-east-2"
}

variable "bucket_name" {
  type    = string
  default = "my-test-bucket"
}
variable "sqs_queue_name" {
  type    = string
  default = "minha-fila" # Corrigido o typo
}
variable "dynamodb_table" {
  type    = string
  default = "fleet-telemetry"
}

variable "localstack_endpoint" {
  type    = string
  default = "http://localhost:4566"
}

variable "s3_localstack_endpoint" {
  type    = string
  default = "http://s3.localhost.localstack.cloud:4566"
}

provider "aws" {
  access_key = var.access_key
  secret_key = var.secret_key
  region     = var.region

  s3_use_path_style           = true
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    apigateway  = var.localstack_endpoint
    dynamodb    = var.localstack_endpoint
    lambda      = var.localstack_endpoint
    sqs         = var.localstack_endpoint
    s3          = var.s3_localstack_endpoint
    rekognition = var.localstack_endpoint
    iam         = var.localstack_endpoint
  }
}

resource "aws_s3_bucket" "photo_bucket" {
  bucket = var.bucket_name
}

resource "aws_sqs_queue" "telemetry_queue" {
  name = var.sqs_queue_name
}

resource "aws_dynamodb_table" "telemetry_table" {
  name         = "fleet-telemetry"
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

resource "aws_iam_role" "lambda_role" {
  name = "lambda_role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

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
  uri                     = "arn:aws:apigateway:us-east-2:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-2:000000000000:function:telemetry_restapi/invocations"
}

resource "aws_api_gateway_deployment" "deployment" {
  rest_api_id = aws_api_gateway_rest_api.telemetry_api.id
  depends_on  = [aws_api_gateway_integration.lambda_integration]
  stage_name  = "prod"
}
