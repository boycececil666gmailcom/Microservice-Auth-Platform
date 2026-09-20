#region Shortener Package
data "archive_file" "shortener_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/shortener_placeholder.zip"

  source {
    content  = "#!/bin/sh\necho 'Placeholder bootstrap'\n"
    filename = "bootstrap"
  }
}
#endregion

#region Shortener Lambda
resource "aws_lambda_function" "shortener" {
  function_name = "${var.app_name}-shortener-${var.environment}"
  role          = aws_iam_role.shortener_lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  memory_size   = 256
  timeout       = 15

  filename         = fileexists("${path.module}/../bin/shortener.zip") ? "${path.module}/../bin/shortener.zip" : data.archive_file.shortener_placeholder.output_path
  source_code_hash = fileexists("${path.module}/../bin/shortener.zip") ? filebase64sha256("${path.module}/../bin/shortener.zip") : data.archive_file.shortener_placeholder.output_base64sha256

  vpc_config {
    subnet_ids         = aws_subnet.private[*].id
    security_group_ids = [aws_security_group.lambda.id]
  }

  environment {
    variables = {
      DATABASE_URL  = "postgresql://${aws_db_instance.postgres.username}:${random_password.db_password.result}@${aws_db_instance.postgres.endpoint}/${aws_db_instance.postgres.db_name}?sslmode=require"
      REDIS_URL     = "redis://${aws_elasticache_cluster.redis.cache_nodes[0].address}:${aws_elasticache_cluster.redis.port}"
      SQS_QUEUE_URL = aws_sqs_queue.url_redirects.url
      ENVIRONMENT   = var.environment
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.shortener,
    aws_iam_role_policy_attachment.shortener_basic,
    aws_iam_role_policy_attachment.shortener_vpc,
    aws_db_instance.postgres,
    aws_elasticache_cluster.redis
  ]
}

resource "aws_cloudwatch_log_group" "shortener" {
  name              = "/aws/lambda/${var.app_name}-shortener-${var.environment}"
  retention_in_days = var.log_retention_days
}
#endregion
