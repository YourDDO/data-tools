locals {
  required_environment_variables = {
    AWS_REGION          = var.aws_region
    CDN_BASE_URL        = var.cdn_base_url
    COMPENDIUM_BASE_URL = var.compendium_base_url
    COMPENDIUM_API_PATH = var.compendium_api_path
    DATA_BUCKET         = var.data_bucket_name
    GAME_VERSION        = var.game_version
    OUTPUT_DIR          = var.output_dir
    PUBLISH_ENABLED     = tostring(var.publish_enabled)
    TRIGGER_SOURCE      = var.trigger_source
  }

  environment_variables = merge(var.environment_variables, local.required_environment_variables)
}

check "environment_variable_names_are_unique" {
  assert {
    condition = length(concat(
      keys(local.environment_variables),
      keys(var.parameter_store_environment_variables),
      keys(var.secrets_manager_environment_variables),
      )) == length(distinct(concat(
        keys(local.environment_variables),
        keys(var.parameter_store_environment_variables),
        keys(var.secrets_manager_environment_variables),
    )))
    error_message = "Plaintext, Parameter Store, and Secrets Manager environment variable names must not overlap."
  }
}

check "connection_is_in_project_region" {
  assert {
    condition     = can(regex("^arn:[^:]+:(?:codeconnections|codestar-connections):${var.aws_region}:", var.repository_connection_arn))
    error_message = "repository_connection_arn must be in the same region as the CodeBuild project."
  }
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/aws/codebuild/${var.project_name}"
  retention_in_days = var.log_retention_days
}


resource "aws_security_group" "this" {
  name                   = "${var.project_name}-codebuild"
  description            = "Restricted egress from the YourDDO data pipeline CodeBuild project"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true
}

resource "aws_vpc_security_group_egress_rule" "compendium" {
  security_group_id            = aws_security_group.this.id
  description                  = "Reach the private Compendium endpoint"
  referenced_security_group_id = var.compendium_security_group_id
  ip_protocol                  = "tcp"
  from_port                    = var.compendium_port
  to_port                      = var.compendium_port
}

resource "aws_vpc_security_group_egress_rule" "https_ipv4" {
  security_group_id = aws_security_group.this.id
  description       = "Reach S3, Logs, source, and Go modules through existing NAT or endpoints"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
}

resource "aws_vpc_security_group_egress_rule" "https_ipv6" {
  security_group_id = aws_security_group.this.id
  description       = "Reach S3, Logs, source, and Go modules through existing IPv6 egress"
  cidr_ipv6         = "::/0"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
}

resource "aws_codebuild_project" "this" {
  name           = var.project_name
  description    = "Build and publish the YourDDO production data release"
  service_role   = aws_iam_role.this.arn
  source_version = var.source_version
  build_timeout  = var.build_timeout_minutes
  queued_timeout = 30

  artifacts {
    type = "NO_ARTIFACTS"
  }

  environment {
    compute_type                = var.compute_type
    image                       = var.image
    image_pull_credentials_type = "CODEBUILD"
    privileged_mode             = false
    type                        = var.environment_type

    dynamic "environment_variable" {
      for_each = local.environment_variables
      content {
        name  = environment_variable.key
        type  = "PLAINTEXT"
        value = environment_variable.value
      }
    }

    dynamic "environment_variable" {
      for_each = var.parameter_store_environment_variables
      content {
        name  = environment_variable.key
        type  = "PARAMETER_STORE"
        value = environment_variable.value.value
      }
    }

    dynamic "environment_variable" {
      for_each = var.secrets_manager_environment_variables
      content {
        name  = environment_variable.key
        type  = "SECRETS_MANAGER"
        value = environment_variable.value.value
      }
    }
  }

  source {
    type            = var.repository_type
    location        = var.repository_url
    git_clone_depth = 1
    buildspec       = var.buildspec

    auth {
      type     = "CODECONNECTIONS"
      resource = var.repository_connection_arn
    }
  }

  logs_config {
    cloudwatch_logs {
      status     = "ENABLED"
      group_name = aws_cloudwatch_log_group.this.name
    }

    s3_logs {
      status = "DISABLED"
    }
  }

  vpc_config {
    vpc_id             = var.vpc_id
    subnets            = var.private_subnet_ids
    security_group_ids = [aws_security_group.this.id]
  }
}
