#region Gateway Outputs
output "api_endpoint" {
  value       = aws_apigatewayv2_stage.default.invoke_url
  description = "Base HTTPS invoke URL for the HTTP API Gateway."
}
#endregion

#region Lambda Outputs
output "shortener_lambda_arn" {
  value       = aws_lambda_function.shortener.arn
  description = "ARN of the URL Shortener Lambda function."
}

output "shortener_lambda_name" {
  value       = aws_lambda_function.shortener.function_name
  description = "Name of the URL Shortener Lambda function."
}

output "analytics_lambda_arn" {
  value       = aws_lambda_function.analytics.arn
  description = "ARN of the Analytics Lambda function."
}

output "analytics_lambda_name" {
  value       = aws_lambda_function.analytics.function_name
  description = "Name of the Analytics Lambda function."
}
#endregion

#region SQS Outputs
output "sqs_queue_url" {
  value       = aws_sqs_queue.url_redirects.url
  description = "URL of the URL redirect events SQS queue."
}

output "sqs_queue_arn" {
  value       = aws_sqs_queue.url_redirects.arn
  description = "ARN of the URL redirect events SQS queue."
}
#endregion

#region Database Outputs
output "rds_endpoint" {
  value       = aws_db_instance.postgres.endpoint
  description = "Endpoint address and port for the PostgreSQL RDS instance."
}

output "rds_database_name" {
  value       = aws_db_instance.postgres.db_name
  description = "Database name on the PostgreSQL RDS instance."
}

output "elasticache_endpoint" {
  value       = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:${aws_elasticache_cluster.redis.port}"
  description = "Connection endpoint for ElastiCache Redis."
}
#endregion

#region Network Outputs
output "vpc_id" {
  value       = aws_vpc.main.id
  description = "ID of the VPC."
}

output "private_subnet_ids" {
  value       = aws_subnet.private[*].id
  description = "IDs of the private subnets."
}
#endregion
