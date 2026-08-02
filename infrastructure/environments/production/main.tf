locals {
  tags = merge(var.tags, {
    Environment = "production"
    ManagedBy   = "Terraform"
    Project     = var.project_name
  })
}

module "data_bucket" {
  source = "../../modules/data-bucket"

  bucket_name                            = var.data_bucket_name
  abort_incomplete_multipart_upload_days = var.abort_incomplete_multipart_upload_days
}

resource "aws_acm_certificate" "cdn" {
  provider          = aws.us_east_1
  domain_name       = var.cdn_domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "cdn_certificate_validation" {
  for_each = {
    for option in aws_acm_certificate.cdn.domain_validation_options :
    option.domain_name => {
      name   = option.resource_record_name
      record = option.resource_record_value
      type   = option.resource_record_type
    }
  }

  zone_id = data.aws_route53_zone.shared.zone_id
  name    = each.value.name
  type    = each.value.type
  ttl     = 300
  records = [each.value.record]
}

resource "aws_acm_certificate_validation" "cdn" {
  provider                = aws.us_east_1
  certificate_arn         = aws_acm_certificate.cdn.arn
  validation_record_fqdns = [for record in aws_route53_record.cdn_certificate_validation : record.fqdn]
}

module "cdn" {
  source = "../../modules/cdn"

  acm_certificate_arn         = aws_acm_certificate_validation.cdn.certificate_arn
  bucket_arn                  = module.data_bucket.arn
  bucket_id                   = module.data_bucket.id
  bucket_regional_domain_name = module.data_bucket.regional_domain_name
  domain_name                 = var.cdn_domain_name
  price_class                 = var.cloudfront_price_class
  web_acl_id                  = var.cloudfront_web_acl_id
}

resource "aws_route53_record" "cdn_ipv4" {
  zone_id = data.aws_route53_zone.shared.zone_id
  name    = var.cdn_domain_name
  type    = "A"

  alias {
    name                   = module.cdn.domain_name
    zone_id                = module.cdn.hosted_zone_id
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "cdn_ipv6" {
  zone_id = data.aws_route53_zone.shared.zone_id
  name    = var.cdn_domain_name
  type    = "AAAA"

  alias {
    name                   = module.cdn.domain_name
    zone_id                = module.cdn.hosted_zone_id
    evaluate_target_health = false
  }
}

module "codebuild" {
  source = "../../modules/codebuild"

  project_name                          = var.project_name
  aws_region                            = var.aws_region
  vpc_id                                = data.aws_vpc.shared.id
  private_subnet_ids                    = [local.codebuild_private_subnet_id]
  compendium_security_group_id          = data.aws_security_group.compendium.id
  compendium_port                       = var.compendium_port
  compendium_api_path                   = var.compendium_api_path
  data_bucket_arn                       = module.data_bucket.arn
  buildspec                             = var.codebuild_buildspec
  repository_url                        = var.repository_url
  repository_connection_arn             = var.repository_connection_arn
  repository_type                       = var.repository_type
  source_version                        = var.source_version
  compendium_base_url                   = var.compendium_base_url
  data_bucket_name                      = module.data_bucket.id
  cdn_base_url                          = "https://${var.cdn_domain_name}"
  game_version                          = var.game_version
  publish_enabled                       = var.publish_enabled
  environment_variables                 = var.codebuild_environment_variables
  parameter_store_environment_variables = var.codebuild_parameter_store_environment_variables
  secrets_manager_environment_variables = var.codebuild_secrets_manager_environment_variables
  build_timeout_minutes                 = var.codebuild_build_timeout_minutes
  log_retention_days                    = var.codebuild_log_retention_days
}

module "scheduler" {
  source = "../../modules/scheduler"

  name                   = "${var.project_name}-schedule"
  codebuild_project_arn  = module.codebuild.codebuild_project_arn
  codebuild_project_name = module.codebuild.codebuild_project_name
  schedule_expression    = var.schedule_expression
  schedule_timezone      = var.schedule_timezone
  enabled                = var.schedule_enabled
}
