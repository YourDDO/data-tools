variable "aws_region" {
  description = "AWS region for data-pipeline resources and shared-resource lookups."
  type        = string

  validation {
    condition     = can(regex("^[a-z]{2}(?:-[a-z0-9]+)+-[0-9]+$", var.aws_region))
    error_message = "aws_region must be a valid AWS region name."
  }
}

variable "project_name" {
  description = "Short name used to identify resources owned by this stack."
  type        = string
  default     = "yourddo-data-tools"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,38}[a-z0-9]$", var.project_name))
    error_message = "project_name must contain 3-40 lowercase letters, numbers, or hyphens."
  }
}

variable "data_bucket_name" {
  description = "Globally unique name for the production data bucket owned by this stack."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$", var.data_bucket_name)) && !strcontains(var.data_bucket_name, "..")
    error_message = "data_bucket_name must be a valid S3 bucket name."
  }
}

variable "abort_incomplete_multipart_upload_days" {
  description = "Days after initiation before S3 aborts an incomplete multipart upload. Completed releases are never expired."
  type        = number
  default     = 7
}

variable "vpc_id" {
  description = "ID of the existing shared VPC; the production stack does not own it."
  type        = string
}

variable "public_subnet_id" {
  description = "ID of the existing public subnet in which the production NAT Gateway is created."
  type        = string
}

variable "private_subnet_ids" {
  description = "IDs of existing private subnets eligible for CodeBuild; one in the NAT Gateway Availability Zone is preferred."
  type        = list(string)

  validation {
    condition     = length(var.private_subnet_ids) > 0 && length(distinct(var.private_subnet_ids)) == length(var.private_subnet_ids)
    error_message = "private_subnet_ids must contain at least one unique subnet ID."
  }
}

variable "route53_zone_id" {
  description = "ID of the existing Route 53 hosted zone; the production stack only owns its CDN records."
  type        = string
}

variable "compendium_security_group_id" {
  description = "Security group ID identifying the existing Compendium service. No rules are managed here."
  type        = string
}

variable "compendium_port" {
  description = "TCP port exposed by the private Compendium endpoint."
  type        = number
  default     = 80

  validation {
    condition     = var.compendium_port >= 1 && var.compendium_port <= 65535
    error_message = "compendium_port must be between 1 and 65535."
  }
}

variable "compendium_base_url" {
  description = "HTTP(S) base URL containing the existing Compendium service's private RFC1918 IPv4 address."
  type        = string

  validation {
    condition = can(regex(
      "^https?://(?:10(?:\\.[0-9]{1,3}){3}|172\\.(?:1[6-9]|2[0-9]|3[01])(?:\\.[0-9]{1,3}){2}|192\\.168(?:\\.[0-9]{1,3}){2})(?::[0-9]{1,5})?/?$",
      var.compendium_base_url,
    ))
    error_message = "compendium_base_url must use the Compendium's private RFC1918 IPv4 address, not a public hostname."
  }
}

variable "compendium_api_path" {
  description = "MediaWiki API path on the existing private Compendium service."
  type        = string
  default     = "/api.php"

  validation {
    condition     = startswith(var.compendium_api_path, "/") && !strcontains(var.compendium_api_path, "?") && !strcontains(var.compendium_api_path, "#")
    error_message = "compendium_api_path must be an absolute URL path without a query or fragment."
  }
}

variable "cdn_domain_name" {
  description = "DNS name for the CloudFront distribution within the existing hosted zone."
  type        = string
  default     = "cdn.yourddo.com"

  validation {
    condition     = can(regex("^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$", var.cdn_domain_name))
    error_message = "cdn_domain_name must be a valid lowercase DNS name."
  }
}

variable "cloudfront_price_class" {
  description = "CloudFront price class."
  type        = string
  default     = "PriceClass_100"

  validation {
    condition     = contains(["PriceClass_100", "PriceClass_200", "PriceClass_All"], var.cloudfront_price_class)
    error_message = "cloudfront_price_class must be PriceClass_100, PriceClass_200, or PriceClass_All."
  }
}

variable "cloudfront_web_acl_id" {
  description = "Optional ARN of an existing AWS WAF web ACL for CloudFront."
  type        = string
  default     = null
  nullable    = true
}

variable "repository_url" {
  description = "HTTPS URL of the source repository used by CodeBuild."
  type        = string

  validation {
    condition     = can(regex("^https://", var.repository_url))
    error_message = "repository_url must be an HTTPS repository URL."
  }
}

variable "repository_connection_arn" {
  description = "ARN of an existing AWS CodeConnections connection authorized to read repository_url."
  type        = string
}

variable "repository_type" {
  description = "CodeBuild source integration used with the managed connection."
  type        = string
  default     = "GITHUB"

  validation {
    condition     = contains(["BITBUCKET", "GITHUB", "GITLAB"], var.repository_type)
    error_message = "repository_type must be BITBUCKET, GITHUB, or GITLAB."
  }
}

variable "source_version" {
  description = "Branch, tag, or commit built by CodeBuild."
  type        = string
  default     = "main"
}

variable "codebuild_buildspec" {
  description = "Repository-relative buildspec used by CodeBuild."
  type        = string
  default     = "v2/buildspec.yml"
}

variable "codebuild_environment_variables" {
  description = "Additional non-secret plaintext environment variables. Use Parameter Store or Secrets Manager for secrets."
  type        = map(string)
  default     = {}
}

variable "codebuild_parameter_store_environment_variables" {
  description = "Parameter Store environment variables. Each entry supplies the CodeBuild value (name) and exact IAM ARN."
  type = map(object({
    value = string
    arn   = string
  }))
  default = {}
}

variable "codebuild_secrets_manager_environment_variables" {
  description = "Secrets Manager environment variables. Each entry supplies the CodeBuild value and exact IAM ARN."
  type = map(object({
    value = string
    arn   = string
  }))
  default = {}
}

variable "codebuild_build_timeout_minutes" {
  description = "Maximum CodeBuild duration in minutes."
  type        = number
  default     = 30
}

variable "codebuild_log_retention_days" {
  description = "CloudWatch Logs retention for CodeBuild output."
  type        = number
  default     = 30
}

variable "game_version" {
  description = "DDO game version assigned to published datasets."
  type        = string

  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+$", var.game_version))
    error_message = "game_version must use numeric major.minor.patch form."
  }
}

variable "publish_enabled" {
  description = "Whether successful CodeBuild runs publish and activate an S3 release."
  type        = bool
  default     = true
}

variable "schedule_expression" {
  description = "EventBridge Scheduler rate or cron expression used to start the production CodeBuild project."
  type        = string
  default     = "cron(0 6 ? * * *)"

  validation {
    condition     = can(regex("^(cron|rate)\\(.+\\)$", var.schedule_expression))
    error_message = "schedule_expression must be an EventBridge Scheduler cron(...) or rate(...) expression."
  }
}

variable "schedule_timezone" {
  description = "IANA timezone used to evaluate schedule_expression."
  type        = string
  default     = "UTC"

  validation {
    condition     = length(trimspace(var.schedule_timezone)) > 0
    error_message = "schedule_timezone must not be empty."
  }
}

variable "schedule_enabled" {
  description = "Whether EventBridge Scheduler may automatically start production builds."
  type        = bool
  default     = false
}

variable "tags" {
  description = "Additional tags for resources owned by the production stack."
  type        = map(string)
  default     = {}
}
