#region Application Resource Group
resource "aws_resourcegroups_group" "application" {
  name        = "${var.app_name}-${var.environment}"
  description = "Application Resource Group for URL Shortener Microservices"

  resource_query {
    query = jsonencode({
      ResourceTypeFilters = ["AWS::AllSupported"]
      TagFilters = [
        {
          Key    = "Application"
          Values = ["${var.app_name}-${var.environment}"]
        }
      ]
    })
  }

  tags = {
    Name = "${var.app_name}-${var.environment}"
  }
}
#endregion
