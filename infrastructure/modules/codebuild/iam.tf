data "aws_caller_identity" "current" {}

data "aws_partition" "current" {}

locals {
  subnet_arns = [
    for subnet_id in var.private_subnet_ids :
    "arn:${data.aws_partition.current.partition}:ec2:${var.aws_region}:${data.aws_caller_identity.current.account_id}:subnet/${subnet_id}"
  ]
}

data "aws_iam_policy_document" "assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["codebuild.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "this" {
  name               = "${var.project_name}-codebuild"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

data "aws_iam_policy_document" "this" {
  statement {
    sid       = "CreateBuildLogGroup"
    actions   = ["logs:CreateLogGroup"]
    resources = [aws_cloudwatch_log_group.this.arn]
  }

  statement {
    sid = "WriteBuildLogs"
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["${aws_cloudwatch_log_group.this.arn}:*"]
  }

  statement {
    sid       = "ListDatasetPublicationPaths"
    actions   = ["s3:ListBucket"]
    resources = [var.data_bucket_arn]

    condition {
      test     = "StringLike"
      variable = "s3:prefix"
      values = [
        "latest.json",
        "releases",
        "releases/*",
      ]
    }
  }

  statement {
    sid = "ReadDatasetPublicationObjects"
    actions = [
      "s3:GetObject",
    ]
    resources = [
      "${var.data_bucket_arn}/latest.json",
      "${var.data_bucket_arn}/releases/*",
    ]
  }

  # The publisher uses conditional PutObject calls and never deletes data.
  statement {
    sid = "PublishDatasetObjects"
    actions = [
      "s3:PutObject",
    ]
    resources = [
      "${var.data_bucket_arn}/latest.json",
      "${var.data_bucket_arn}/releases/*",
    ]
  }

  statement {
    sid = "ReadSourceThroughConnection"

    actions = [
      "codeconnections:UseConnection",
      "codeconnections:GetConnection",
      "codeconnections:GetConnectionToken",
    ]

    resources = [
      var.repository_connection_arn
    ]
  }

  dynamic "statement" {
    for_each = length(var.parameter_store_environment_variables) == 0 ? [] : [true]

    content {
      sid       = "ReadExplicitParameters"
      actions   = ["ssm:GetParameter", "ssm:GetParameters"]
      resources = [for setting in values(var.parameter_store_environment_variables) : setting.arn]
    }
  }

  dynamic "statement" {
    for_each = length(var.secrets_manager_environment_variables) == 0 ? [] : [true]

    content {
      sid       = "ReadExplicitSecrets"
      actions   = ["secretsmanager:GetSecretValue"]
      resources = [for setting in values(var.secrets_manager_environment_variables) : setting.arn]
    }
  }

  # These EC2 APIs either do not support resource-level permissions or must
  # operate on the ephemeral network interface created for each build.
  statement {
    sid = "ManageVpcNetworkInterfaces"
    actions = [
      "ec2:CreateNetworkInterface",
      "ec2:DeleteNetworkInterface",
      "ec2:DescribeDhcpOptions",
      "ec2:DescribeNetworkInterfaces",
      "ec2:DescribeSecurityGroups",
      "ec2:DescribeSubnets",
      "ec2:DescribeVpcs",
    ]
    resources = ["*"]
  }

  statement {
    sid       = "AllowCodeBuildNetworkInterfacePermission"
    actions   = ["ec2:CreateNetworkInterfacePermission"]
    resources = ["arn:${data.aws_partition.current.partition}:ec2:${var.aws_region}:${data.aws_caller_identity.current.account_id}:network-interface/*"]

    condition {
      test     = "StringEquals"
      variable = "ec2:AuthorizedService"
      values   = ["codebuild.amazonaws.com"]
    }

    condition {
      test     = "ArnEquals"
      variable = "ec2:Subnet"
      values   = local.subnet_arns
    }
  }
}

resource "aws_iam_role_policy" "this" {
  name   = "${var.project_name}-codebuild"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.this.json
}
