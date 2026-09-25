output "api_endpoint" {
  value       = aws_apigatewayv2_stage.default.invoke_url
  description = "Base HTTPS invoke URL for the HTTP API."
}

output "shortener_lambda_name" {
  value       = aws_lambda_function.shortener.function_name
  description = "Shortener Lambda function name."
}

output "analytics_lambda_name" {
  value       = aws_lambda_function.analytics.function_name
  description = "Analytics Lambda function name."
}

output "urls_table_name" {
  value       = aws_dynamodb_table.urls.name
  description = "DynamoDB URL table name."
}

output "analytics_table_name" {
  value       = aws_dynamodb_table.analytics.name
  description = "DynamoDB analytics table name."
}

output "sqs_queue_url" {
  value       = aws_sqs_queue.url_redirects.url
  description = "Redirect event queue URL."
}

output "application_arn" {
  value       = aws_resourcegroups_group.application.arn
  description = "AWS Application Resource Group ARN."
}
