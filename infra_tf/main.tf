#region Terraform Config
terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.4"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}
#endregion

#region AWS Provider
provider "aws" {
  region  = var.aws_region
  profile = "terraform-deployer"

  default_tags {
    tags = {
      Project        = var.app_name
      Environment    = var.environment
      ManagedBy      = "Terraform"
      Application    = "${var.app_name}-${var.environment}"
      awsApplication = "arn:aws:resource-groups:${var.aws_region}:${var.aws_account_id}:group/${var.app_name}-${var.environment}"
    }
  }
}
#endregion
