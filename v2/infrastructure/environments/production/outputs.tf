output "data_bucket_name" {
  description = "S3 bucket consumed through the DATA_BUCKET application setting."
  value       = module.data_bucket.id
}

output "bucket_name" {
  description = "Private production dataset bucket name."
  value       = module.data_bucket.id
}

output "cdn_base_url" {
  description = "HTTPS base URL consumed through the CDN_BASE_URL application setting."
  value       = "https://${var.cdn_domain_name}"
}

output "cdn_url" {
  description = "Public HTTPS URL for the production dataset CDN."
  value       = "https://${var.cdn_domain_name}"
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID."
  value       = module.cdn.distribution_id
}

output "cloudfront_domain_name" {
  description = "AWS-assigned CloudFront distribution domain name."
  value       = module.cdn.domain_name
}

output "codebuild_project_name" {
  description = "Production CodeBuild project name."
  value       = module.codebuild.codebuild_project_name
}

output "codebuild_project_arn" {
  description = "Production CodeBuild project ARN."
  value       = module.codebuild.codebuild_project_arn
}

output "codebuild_service_role_arn" {
  description = "Least-privilege CodeBuild service role ARN."
  value       = module.codebuild.codebuild_service_role_arn
}

output "codebuild_log_group_name" {
  description = "Dedicated CloudWatch log group for CodeBuild output."
  value       = module.codebuild.codebuild_log_group_name
}

output "codebuild_security_group_id" {
  description = "Security group that the shared Compendium stack must permit as required."
  value       = module.codebuild.codebuild_security_group_id
}

output "nat_gateway_id" {
  description = "ID of the single public NAT Gateway used for CodeBuild internet egress."
  value       = aws_nat_gateway.codebuild.id
}

output "nat_gateway_elastic_ip" {
  description = "Elastic IPv4 address associated with the CodeBuild NAT Gateway."
  value       = aws_eip.nat.public_ip
}

output "nat_gateway_public_subnet_id" {
  description = "ID of the existing public subnet containing the NAT Gateway."
  value       = data.aws_subnet.public.id
}

output "codebuild_private_route_table_id" {
  description = "ID of the existing private route table whose default route targets the NAT Gateway."
  value       = data.aws_route_table.codebuild_private.id
}

output "codebuild_private_subnet_id" {
  description = "ID of the existing private subnet selected for CodeBuild."
  value       = local.codebuild_private_subnet_id
}

output "schedule_arn" {
  description = "EventBridge Scheduler schedule that starts the production CodeBuild project."
  value       = module.scheduler.schedule_arn
}

output "referenced_shared_infrastructure" {
  description = "Existing resources read, but not owned, by this stack."
  value = {
    account_id                   = data.aws_caller_identity.current.account_id
    vpc_id                       = data.aws_vpc.shared.id
    public_subnet_id             = data.aws_subnet.public.id
    private_subnet_ids           = sort([for subnet in data.aws_subnet.private : subnet.id])
    route53_zone_id              = data.aws_route53_zone.shared.zone_id
    compendium_security_group_id = data.aws_security_group.compendium.id
    source_connection_arn        = var.repository_connection_arn
  }
}
