output "sqs_queue_url" {
  value = aws_sqs_queue.minha_fila.id
}

output "s3_test_bucket" {
  value = aws_s3_bucket.my_test_bucket.bucket
}

output "s3_lambda_bucket" {
  value = aws_s3_bucket.my_lambda_bucket.bucket
}

output "dynamodb_fleet_telemetry" {
  value = aws_dynamodb_table.fleet_telemetry.name
}
