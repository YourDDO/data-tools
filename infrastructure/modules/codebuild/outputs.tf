output "codebuild_project_name" {
  description = "CodeBuild project name."
  value       = aws_codebuild_project.this.name
}

output "codebuild_project_arn" {
  description = "CodeBuild project ARN."
  value       = aws_codebuild_project.this.arn
}

output "codebuild_service_role_arn" {
  description = "CodeBuild execution role ARN."
  value       = aws_iam_role.this.arn
}

output "codebuild_log_group_name" {
  description = "Dedicated CloudWatch Logs group for CodeBuild output."
  value       = aws_cloudwatch_log_group.this.name
}

output "codebuild_security_group_id" {
  description = "CodeBuild security group ID for shared-service ingress rules."
  value       = aws_security_group.this.id
}
