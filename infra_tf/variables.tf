variable "aws_region" {
  type        = string
  description = "AWS region for provisioning resources."
  default     = "us-east-1"
}

variable "aws_profile" {
  type        = string
  description = "Optional local AWS profile. CI should use its attached IAM role."
  default     = ""
}

variable "aws_account_id" {
  type        = string
  description = "Deprecated and ignored; retained so older local tfvars remain compatible."
  default     = ""
}

variable "environment" {
  type        = string
  description = "Deployment environment."
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be dev, staging, or prod."
  }
}

variable "app_name" {
  type        = string
  description = "Application name prefix used across resources."
  default     = "url-shortener"
}

variable "log_retention_days" {
  type        = number
  description = "CloudWatch log retention in days."
  default     = 14

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365], var.log_retention_days)
    error_message = "log_retention_days must be a CloudWatch-supported value."
  }
}

variable "api_throttle_rate" {
  type        = number
  description = "Steady-state API Gateway requests per second."
  default     = 50
}

variable "api_throttle_burst" {
  type        = number
  description = "API Gateway burst capacity."
  default     = 100
}

variable "alarm_topic_arn" {
  type        = string
  description = "Optional SNS topic ARN notified when an alarm enters ALARM state."
  default     = ""
}
