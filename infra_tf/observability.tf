resource "aws_cloudwatch_metric_alarm" "redirect_dlq" {
  alarm_name          = "${var.app_name}-redirect-dlq-${var.environment}"
  alarm_description   = "Redirect events are reaching the dead-letter queue."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_topic_arn == "" ? [] : [var.alarm_topic_arn]

  dimensions = {
    QueueName = aws_sqs_queue.url_redirects_dlq.name
  }
}

resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  for_each = {
    shortener = aws_lambda_function.shortener.function_name
    analytics = aws_lambda_function.analytics.function_name
  }

  alarm_name          = "${var.app_name}-${each.key}-errors-${var.environment}"
  alarm_description   = "${each.key} Lambda reported errors."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_topic_arn == "" ? [] : [var.alarm_topic_arn]

  dimensions = {
    FunctionName = each.value
  }
}
