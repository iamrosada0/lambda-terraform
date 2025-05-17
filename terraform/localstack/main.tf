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

variable "bucket_name" {
  type    = string
  default = "my-test-bucket"
}
variable "sqs_queue_name" {
  type    = string
  default = "my-custom-sqs-queue"
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


# Rest of your main.tf (SQS, DynamoDB, S3, Lambda, API Gateway resources)
# Ensure resource names match TF_VAR_bucket_name and TF_VAR_sqs_queue_name
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
        Action   = ["s3:PutObject", "s3:GetObject"]
        Resource = "${aws_s3_bucket.photo_bucket.arn}/*"
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
resource "aws_lambda_function" "processor_lambda" {
  filename         = "processor_lambda.zip"
  function_name    = "telemetry_processor"
  role             = aws_iam_role.lambda_role.arn
  handler          = "main"
  runtime          = "go1.x"
  source_code_hash = filebase64sha256("processor_lambda.zip")
  timeout          = 120
  memory_size      = 512

  environment {
    variables = {
      AWS_DEFAULT_REGION                 = var.region
      LOCALSTACK_ENDPOINT                = var.localstack_endpoint
      S3_ENDPOINT                        = var.s3_localstack_endpoint
      S3_BUCKET                          = var.bucket_name
      DYNAMODB_TABLE                     = var.dynamodb_table
      SQS_QUEUE_URL                      = var.localstack_endpoint
      LAMBDA_RUNTIME_ENVIRONMENT_TIMEOUT = "120"
    }
  }
}




resource "aws_lambda_event_source_mapping" "sqs_trigger" {
  event_source_arn = aws_sqs_queue.telemetry_queue.arn
  function_name    = aws_lambda_function.processor_lambda.arn
}

resource "aws_lambda_function" "restapi_lambda" {
  filename         = "restapi_lambda.zip"
  function_name    = "telemetry_restapi"
  role             = aws_iam_role.lambda_role.arn
  handler          = "main"
  runtime          = "go1.x"
  source_code_hash = filebase64sha256("restapi_lambda.zip")

  environment {
    variables = {
      AWS_DEFAULT_REGION  = var.region
      LOCALSTACK_ENDPOINT = var.localstack_endpoint
      S3_ENDPOINT         = var.s3_localstack_endpoint
      S3_BUCKET           = var.bucket_name
      DYNAMODB_TABLE      = var.dynamodb_table
      SQS_QUEUE_URL       = aws_sqs_queue.telemetry_queue.id
    }
  }
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
