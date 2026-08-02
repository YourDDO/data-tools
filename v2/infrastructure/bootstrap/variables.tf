variable "aws_region" {
  description = "AWS region in which to create the Terraform state bucket."
  type        = string

  validation {
    condition     = can(regex("^[a-z]{2}(?:-[a-z0-9]+)+-[0-9]+$", var.aws_region))
    error_message = "aws_region must be a valid AWS region name."
  }
}

variable "state_bucket_name" {
  description = "Globally unique name for the Terraform state S3 bucket."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$", var.state_bucket_name)) && !strcontains(var.state_bucket_name, "..")
    error_message = "state_bucket_name must be a valid S3 bucket name."
  }
}

variable "tags" {
  description = "Tags applied to the state bucket."
  type        = map(string)
  default = {
    Environment = "shared"
    ManagedBy   = "Terraform"
    Project     = "yourddo-data-tools"
  }
}
