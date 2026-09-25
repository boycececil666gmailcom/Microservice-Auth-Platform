#region SQS Queues
resource "aws_sqs_queue" "url_redirects_dlq" {
  name                      = "${var.app_name}-redirects-dlq-${var.environment}"
  message_retention_seconds = 1209600 # 14 days
  sqs_managed_sse_enabled   = true
}

resource "aws_sqs_queue" "url_redirects" {
  name                       = "${var.app_name}-redirects-${var.environment}"
  message_retention_seconds  = 86400 # 1 day
  visibility_timeout_seconds = 90
  sqs_managed_sse_enabled    = true

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.url_redirects_dlq.arn
    maxReceiveCount     = 3
  })
}
#endregion
