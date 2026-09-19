#region Core Variables
variable "aws_region" {
  type        = string
  description = "AWS region for provisioning resources."
  default     = "us-east-1"
}

variable "environment" {
  type        = string
  description = "Deployment environment name (e.g. dev, prod)."
  default     = "dev"
}

variable "app_name" {
  type        = string
  description = "Application name prefix used across resources."
  default     = "url-shortener"
}

variable "log_retention_days" {
  type        = number
  description = "Retention period in days for Lambda CloudWatch logs."
  default     = 14
}
#endregion
