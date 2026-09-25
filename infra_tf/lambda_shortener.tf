resource "aws_lambda_function" "shortener" {
  function_name = "${var.app_name}-shortener-${var.environment}"
  role          = aws_iam_role.shortener_lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  memory_size   = 128
  timeout       = 10

  filename         = "${path.module}/../bin/shortener.zip"
  source_code_hash = filebase64sha256("${path.module}/../bin/shortener.zip")

  environment {
    variables = {
      URLS_TABLE    = aws_dynamodb_table.urls.name
      SQS_QUEUE_URL = aws_sqs_queue.url_redirects.url
      ENVIRONMENT   = var.environment
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.shortener,
    aws_iam_role_policy_attachment.shortener_basic
  ]
}

resource "aws_cloudwatch_log_group" "shortener" {
  name              = "/aws/lambda/${var.app_name}-shortener-${var.environment}"
  retention_in_days = var.log_retention_days
}
