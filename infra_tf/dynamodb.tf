resource "aws_dynamodb_table" "urls" {
  name         = "${var.app_name}-urls-${var.environment}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "short_url"

  attribute {
    name = "short_url"
    type = "S"
  }

  deletion_protection_enabled = local.is_production

  point_in_time_recovery {
    enabled = local.is_production
  }

  server_side_encryption {
    enabled = true
  }
}

resource "aws_dynamodb_table" "analytics" {
  name         = "${var.app_name}-analytics-${var.environment}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "metric_key"

  attribute {
    name = "metric_key"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  deletion_protection_enabled = local.is_production

  point_in_time_recovery {
    enabled = local.is_production
  }

  server_side_encryption {
    enabled = true
  }
}
