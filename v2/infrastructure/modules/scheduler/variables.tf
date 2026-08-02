variable "name" {
  description = "Scheduler group, schedule, and IAM role name prefix."
  type        = string
}

variable "codebuild_project_arn" {
  description = "ARN of the CodeBuild project the schedule may start."
  type        = string
}

variable "codebuild_project_name" {
  description = "Name passed to the CodeBuild StartBuild API."
  type        = string
}

variable "schedule_expression" {
  description = "EventBridge Scheduler rate or cron expression."
  type        = string
}

variable "schedule_timezone" {
  description = "IANA timezone used to evaluate the expression."
  type        = string
  default     = "UTC"
}

variable "enabled" {
  description = "Whether the schedule is enabled."
  type        = bool
  default     = false
}
