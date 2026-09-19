#region Lambda Assume Role
data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}
#endregion

#region Shortener IAM Role
resource "aws_iam_role" "shortener_lambda" {
  name               = "${var.app_name}-shortener-role-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

resource "aws_iam_role_policy_attachment" "shortener_basic" {
  role       = aws_iam_role.shortener_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "shortener_sqs" {
  name = "${var.app_name}-shortener-sqs-publish"
  role = aws_iam_role.shortener_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["sqs:SendMessage", "sqs:GetQueueUrl"]
        Resource = aws_sqs_queue.url_redirects.arn
      }
    ]
  })
}
#endregion

#region Analytics IAM Role
resource "aws_iam_role" "analytics_lambda" {
  name               = "${var.app_name}-analytics-role-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

resource "aws_iam_role_policy_attachment" "analytics_basic" {
  role       = aws_iam_role.analytics_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "analytics_sqs" {
  name = "${var.app_name}-analytics-sqs-consume"
  role = aws_iam_role.analytics_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes"
        ]
        Resource = aws_sqs_queue.url_redirects.arn
      }
    ]
  })
}
#endregion
