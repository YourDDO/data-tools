variable "project_name" {
  description = "CodeBuild project and IAM role name prefix."
  type        = string
}

variable "vpc_id" {
  description = "Existing VPC in which CodeBuild runs."
  type        = string
}

variable "private_subnet_ids" {
  description = "Existing private subnet IDs attached to CodeBuild."
  type        = list(string)
}

variable "data_bucket_arn" {
  description = "ARN of the data bucket whose release paths the publisher may read and write."
  type        = string
}

variable "aws_region" {
  description = "AWS region containing the VPC and CodeBuild project."
  type        = string
}

variable "repository_url" {
  description = "HTTPS URL of the CodeBuild source repository."
  type        = string

  validation {
    condition     = can(regex("^https://", var.repository_url))
    error_message = "repository_url must be an HTTPS repository URL."
  }
}

variable "repository_connection_arn" {
  description = "ARN of an existing AWS CodeConnections connection used to read the source repository."
  type        = string

  validation {
    condition     = can(regex("^arn:[^:]+:(?:codeconnections|codestar-connections):[^:]+:[0-9]{12}:connection/[A-Za-z0-9-]+$", var.repository_connection_arn))
    error_message = "repository_connection_arn must be an AWS CodeConnections connection ARN."
  }
}

variable "repository_type" {
  description = "CodeBuild source type used with the managed CodeConnections connection."
  type        = string
  default     = "GITHUB"

  validation {
    condition     = contains(["BITBUCKET", "GITHUB", "GITLAB"], var.repository_type)
    error_message = "repository_type must be BITBUCKET, GITHUB, or GITLAB."
  }
}

variable "source_version" {
  description = "Source branch, tag, or commit."
  type        = string
  default     = "main"
}

variable "buildspec" {
  description = "Repository-relative buildspec path."
  type        = string
  default     = "buildspec.yml"
}

variable "compendium_base_url" {
  description = "Base URL of the existing Compendium service."
  type        = string
}

variable "compendium_api_path" {
  description = "MediaWiki API path appended to compendium_base_url."
  type        = string
  default     = "/api.php"

  validation {
    condition     = startswith(var.compendium_api_path, "/") && !strcontains(var.compendium_api_path, "?") && !strcontains(var.compendium_api_path, "#")
    error_message = "compendium_api_path must be an absolute URL path without a query or fragment."
  }
}

variable "data_bucket_name" {
  description = "Data bucket exposed to the application."
  type        = string
}

variable "cdn_base_url" {
  description = "CloudFront base URL exposed to the application."
  type        = string
}

variable "game_version" {
  description = "DDO game version assigned to releases."
  type        = string

  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+\\.[0-9]+$", var.game_version))
    error_message = "game_version must use numeric major.minor.patch form."
  }
}

variable "publish_enabled" {
  description = "Whether successful builds publish and activate an S3 release."
  type        = bool
  default     = true
}

variable "output_dir" {
  description = "Ephemeral parent directory used for generated pipeline data."
  type        = string
  default     = "/tmp/yourddo-data"
}

variable "trigger_source" {
  description = "Human-readable source recorded for manual builds."
  type        = string
  default     = "manual"
}

variable "environment_variables" {
  description = "Additional non-secret plaintext environment variables."
  type        = map(string)
  default     = {}

  validation {
    condition     = alltrue([for name in keys(var.environment_variables) : can(regex("^[A-Za-z_][A-Za-z0-9_]*$", name))])
    error_message = "environment_variables keys must be valid environment variable names."
  }
}

variable "parameter_store_environment_variables" {
  description = "Parameter Store-backed environment variables and the exact parameter ARNs CodeBuild may read."
  type = map(object({
    value = string
    arn   = string
  }))
  default = {}

  validation {
    condition = alltrue([
      for name, setting in var.parameter_store_environment_variables :
      can(regex("^[A-Za-z_][A-Za-z0-9_]*$", name)) && can(regex("^arn:[^:]+:ssm:[^:]+:[0-9]{12}:parameter/", setting.arn))
    ])
    error_message = "Parameter Store entries require valid environment variable names and parameter ARNs."
  }
}

variable "secrets_manager_environment_variables" {
  description = "Secrets Manager-backed environment variables and the exact secret ARNs CodeBuild may read."
  type = map(object({
    value = string
    arn   = string
  }))
  default = {}

  validation {
    condition = alltrue([
      for name, setting in var.secrets_manager_environment_variables :
      can(regex("^[A-Za-z_][A-Za-z0-9_]*$", name)) && can(regex("^arn:[^:]+:secretsmanager:[^:]+:[0-9]{12}:secret:", setting.arn))
    ])
    error_message = "Secrets Manager entries require valid environment variable names and secret ARNs."
  }
}

variable "compute_type" {
  description = "CodeBuild compute type."
  type        = string
  default     = "BUILD_GENERAL1_SMALL"
}

variable "image" {
  description = "CodeBuild managed image."
  type        = string
  default     = "aws/codebuild/amazonlinux-aarch64-standard:3.0"
}

variable "environment_type" {
  description = "CodeBuild environment architecture. The default is Linux ARM."
  type        = string
  default     = "ARM_CONTAINER"

  validation {
    condition     = contains(["ARM_CONTAINER", "LINUX_CONTAINER"], var.environment_type)
    error_message = "environment_type must be ARM_CONTAINER or LINUX_CONTAINER."
  }
}

variable "build_timeout_minutes" {
  description = "Maximum build duration in minutes."
  type        = number
  default     = 30

  validation {
    condition     = var.build_timeout_minutes >= 5 && var.build_timeout_minutes <= 2160
    error_message = "build_timeout_minutes must be between 5 and 2160."
  }
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention for CodeBuild output."
  type        = number
  default     = 30

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653], var.log_retention_days)
    error_message = "log_retention_days must be a CloudWatch Logs-supported retention value."
  }
}

variable "compendium_security_group_id" {
  description = "Security group attached to the existing private Compendium endpoint."
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
