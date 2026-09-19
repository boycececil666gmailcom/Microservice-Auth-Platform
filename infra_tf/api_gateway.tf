#region HTTP API Gateway
resource "aws_apigatewayv2_api" "http_api" {
  name          = "${var.app_name}-api-${var.environment}"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.http_api.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gw.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      routeKey       = "$context.routeKey"
      status         = "$context.status"
      protocol       = "$context.protocol"
      responseLength = "$context.responseLength"
    })
  }
}

resource "aws_cloudwatch_log_group" "api_gw" {
  name              = "/aws/apigateway/${var.app_name}-api-${var.environment}"
  retention_in_days = var.log_retention_days
}
#endregion

#region Lambda Integrations
resource "aws_apigatewayv2_integration" "shortener" {
  api_id                 = aws_apigatewayv2_api.http_api.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.shortener.invoke_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_integration" "analytics" {
  api_id                 = aws_apigatewayv2_api.http_api.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.analytics.invoke_arn
  payload_format_version = "2.0"
}
#endregion

#region API Routes
resource "aws_apigatewayv2_route" "shorten" {
  api_id    = aws_apigatewayv2_api.http_api.id
  route_key = "POST /api/v1/shorten"
  target    = "integrations/${aws_apigatewayv2_integration.shortener.id}"
}

resource "aws_apigatewayv2_route" "lookup" {
  api_id    = aws_apigatewayv2_api.http_api.id
  route_key = "GET /api/v1/urls/{shortURL}"
  target    = "integrations/${aws_apigatewayv2_integration.shortener.id}"
}

resource "aws_apigatewayv2_route" "redirect" {
  api_id    = aws_apigatewayv2_api.http_api.id
  route_key = "GET /r/{shortURL}"
  target    = "integrations/${aws_apigatewayv2_integration.shortener.id}"
}

resource "aws_apigatewayv2_route" "health" {
  api_id    = aws_apigatewayv2_api.http_api.id
  route_key = "GET /health"
  target    = "integrations/${aws_apigatewayv2_integration.shortener.id}"
}

resource "aws_apigatewayv2_route" "stats_v1" {
  api_id    = aws_apigatewayv2_api.http_api.id
  route_key = "GET /api/v1/analytics/stats"
  target    = "integrations/${aws_apigatewayv2_integration.analytics.id}"
}

resource "aws_apigatewayv2_route" "stats" {
  api_id    = aws_apigatewayv2_api.http_api.id
  route_key = "GET /stats"
  target    = "integrations/${aws_apigatewayv2_integration.analytics.id}"
}
#endregion

#region Gateway Permissions
resource "aws_lambda_permission" "shortener_gw" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.shortener.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.http_api.execution_arn}/*/*"
}

resource "aws_lambda_permission" "analytics_gw" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.analytics.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.http_api.execution_arn}/*/*"
}
#endregion
