resource "aws_scheduler_schedule_group" "this" {
  name = var.name
}

data "aws_iam_policy_document" "assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "this" {
  name               = "${var.name}-scheduler"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

data "aws_iam_policy_document" "start_build" {
  statement {
    actions   = ["codebuild:StartBuild"]
    resources = [var.codebuild_project_arn]
  }
}

resource "aws_iam_role_policy" "start_build" {
  name   = "${var.name}-start-build"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.start_build.json
}

resource "aws_scheduler_schedule" "this" {
  name                         = var.name
  group_name                   = aws_scheduler_schedule_group.this.name
  schedule_expression          = var.schedule_expression
  schedule_expression_timezone = var.schedule_timezone
  state                        = var.enabled ? "ENABLED" : "DISABLED"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = "arn:aws:scheduler:::aws-sdk:codebuild:startBuild"
    role_arn = aws_iam_role.this.arn
    input = jsonencode({
      ProjectName = var.codebuild_project_name
      EnvironmentVariablesOverride = [{
        Name  = "TRIGGER_SOURCE"
        Type  = "PLAINTEXT"
        Value = "scheduled"
      }]
    })

    retry_policy {
      maximum_event_age_in_seconds = 3600
      maximum_retry_attempts       = 2
    }
  }
}
