#region Analytics Package
data "archive_file" "analytics_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/analytics_placeholder.zip"

  source {
    content  = "#!/bin/sh\necho 'Placeholder bootstrap'\n"
    filename = "bootstrap"
  }
}
#endregion

#region Analytics Lambda
resource "aws_lambda_function" "analytics" {
  function_name = "${var.app_name}-analytics-${var.environment}"
  role          = aws_iam_role.analytics_lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  memory_size   = 256
  timeout       = 15

  filename         = fileexists("${path.module}/../bin/analytics.zip") ? "${path.module}/../bin/analytics.zip" : data.archive_file.analytics_placeholder.output_path
  source_code_hash = fileexists("${path.module}/../bin/analytics.zip") ? filebase64sha256("${path.module}/../bin/analytics.zip") : data.archive_file.analytics_placeholder.output_base64sha256

  vpc_config {
    subnet_ids         = aws_subnet.private[*].id
    security_group_ids = [aws_security_group.lambda.id]
  }

  environment {
    variables = {
      REDIS_URL     = "redis://${aws_elasticache_cluster.redis.cache_nodes[0].address}:${aws_elasticache_cluster.redis.port}"
      SQS_QUEUE_URL = aws_sqs_queue.url_redirects.url
      ENVIRONMENT   = var.environment
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.analytics,
    aws_iam_role_policy_attachment.analytics_basic,
    aws_iam_role_policy_attachment.analytics_vpc,
    aws_elasticache_cluster.redis
  ]
}

resource "aws_cloudwatch_log_group" "analytics" {
  name              = "/aws/lambda/${var.app_name}-analytics-${var.environment}"
  retention_in_days = var.log_retention_days
}
#endregion

#region SQS Trigger
resource "aws_lambda_event_source_mapping" "analytics_sqs" {
  event_source_arn = aws_sqs_queue.url_redirects.arn
  function_name    = aws_lambda_function.analytics.arn
  batch_size       = 10
  enabled          = true
}
#endregion
