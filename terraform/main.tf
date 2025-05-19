terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true
  endpoints {
    s3          = "http://localhost:4566"
    sqs         = "http://localhost:4566"
    dynamodb    = "http://localhost:4566"
    lambda      = "http://localhost:4566"
    rekognition = "http://localhost:4566"
  }
}

resource "aws_sqs_queue" "minha_fila" {
  name = "minha-fila"
}

resource "aws_s3_bucket" "my_test_bucket" {
  bucket = "my-test-bucket"
}

resource "aws_s3_bucket" "my_lambda_bucket" {
  bucket = "my-lambda-bucket"
}

resource "aws_s3_object" "lambda_zip" {
  bucket     = aws_s3_bucket.my_lambda_bucket.bucket
  key        = "go-lambda-localstack-test.zip"
  source     = "../go-lambda-localstack-test.zip"
  depends_on = [aws_s3_bucket.my_lambda_bucket]
}

resource "aws_dynamodb_table" "fleet_telemetry" {
  name           = "fleet-telemetry"
  billing_mode   = "PROVISIONED"
  read_capacity  = 5
  write_capacity = 5
  hash_key       = "device_id"
  range_key      = "timestamp"

  attribute {
    name = "device_id"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "S"
  }
}
