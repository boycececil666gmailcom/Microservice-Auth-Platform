resource "aws_lambda_function" "analytics" {
  function_name = "${var.app_name}-analytics-${var.environment}"
  role          = aws_iam_role.analytics_lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  memory_size   = 128
  timeout       = 15

  filename         = "${path.module}/../bin/analytics.zip"
  source_code_hash = filebase64sha256("${path.module}/../bin/analytics.zip")

  environment {
    variables = {
      ANALYTICS_TABLE    = aws_dynamodb_table.analytics.name
      DEDUPE_TTL_SECONDS = "1296000"
      ENVIRONMENT        = var.environment
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.analytics,
    aws_iam_role_policy_attachment.analytics_basic
  ]
}

resource "aws_cloudwatch_log_group" "analytics" {
  name              = "/aws/lambda/${var.app_name}-analytics-${var.environment}"
  retention_in_days = var.log_retention_days
}

resource "aws_lambda_event_source_mapping" "analytics_sqs" {
  event_source_arn        = aws_sqs_queue.url_redirects.arn
  function_name           = aws_lambda_function.analytics.arn
  batch_size              = 10
  function_response_types = ["ReportBatchItemFailures"]
  enabled                 = true
}
