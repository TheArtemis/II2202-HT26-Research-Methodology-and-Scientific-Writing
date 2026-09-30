data "aws_iam_policy_document" "kill_lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "kill_lambda" {
  name               = "${var.project}-${var.campaign_id}-kill"
  assume_role_policy = data.aws_iam_policy_document.kill_lambda_assume.json
}

data "aws_iam_policy_document" "kill_lambda" {
  statement {
    sid    = "Logs"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["arn:aws:logs:${data.aws_region.current.name}:${local.account_id}:*"]
  }

  statement {
    sid    = "DescribeCampaignInstances"
    effect = "Allow"
    actions = [
      "ec2:DescribeInstances",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "TerminateCampaignInstances"
    effect = "Allow"
    actions = [
      "ec2:TerminateInstances",
    ]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "ec2:ResourceTag/Project"
      values   = [var.project]
    }
    condition {
      test     = "StringEquals"
      variable = "ec2:ResourceTag/Campaign"
      values   = [var.campaign_id]
    }
  }
}

resource "aws_iam_role_policy" "kill_lambda" {
  name   = "terminate-campaign-instances"
  role   = aws_iam_role.kill_lambda.id
  policy = data.aws_iam_policy_document.kill_lambda.json
}

data "archive_file" "kill_lambda" {
  type        = "zip"
  output_path = "${path.module}/.generated/kill_lambda.zip"

  source {
    content  = <<-PY
      import boto3
      import os


      def handler(event, context):
          project = os.environ["PROJECT"]
          campaign = os.environ["CAMPAIGN_ID"]
          ec2 = boto3.client("ec2")
          filters = [
              {"Name": "tag:Project", "Values": [project]},
              {"Name": "tag:Campaign", "Values": [campaign]},
              {
                  "Name": "instance-state-name",
                  "Values": ["pending", "running", "stopping", "stopped"],
              },
          ]
          reservations = ec2.describe_instances(Filters=filters)["Reservations"]
          ids = [
              instance["InstanceId"]
              for reservation in reservations
              for instance in reservation["Instances"]
          ]
          if ids:
              ec2.terminate_instances(InstanceIds=ids)
          return {"terminated": ids, "project": project, "campaign": campaign}
    PY
    filename = "index.py"
  }
}

resource "aws_lambda_function" "kill" {
  function_name = "${var.project}-${var.campaign_id}-kill"
  role          = aws_iam_role.kill_lambda.arn
  handler       = "index.handler"
  runtime       = "python3.12"
  timeout       = 60
  filename      = data.archive_file.kill_lambda.output_path
  source_code_hash = data.archive_file.kill_lambda.output_base64sha256

  environment {
    variables = {
      PROJECT     = var.project
      CAMPAIGN_ID = var.campaign_id
    }
  }

  depends_on = [aws_iam_role_policy.kill_lambda]
}

data "aws_iam_policy_document" "scheduler_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name               = "${var.project}-${var.campaign_id}-scheduler"
  assume_role_policy = data.aws_iam_policy_document.scheduler_assume.json
}

data "aws_iam_policy_document" "scheduler_invoke" {
  statement {
    effect    = "Allow"
    actions   = ["lambda:InvokeFunction"]
    resources = [aws_lambda_function.kill.arn]
  }
}

resource "aws_iam_role_policy" "scheduler_invoke" {
  name   = "invoke-kill-lambda"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.scheduler_invoke.json
}

resource "time_static" "fleet_start" {}

locals {
  kill_at = timeadd(time_static.fleet_start.rfc3339, "${var.max_runtime_hours}h")
  # EventBridge Scheduler one-time at() expression (UTC, no timezone suffix).
  kill_schedule_at = formatdate("YYYY-MM-DD'T'HH:mm:ss", local.kill_at)
}

resource "aws_scheduler_schedule" "kill_fleet" {
  name                         = "${var.project}-${var.campaign_id}-kill"
  description                  = "Auto-terminate campaign EC2 after max_runtime_hours"
  schedule_expression          = "at(${local.kill_schedule_at})"
  schedule_expression_timezone = "UTC"
  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_function.kill.arn
    role_arn = aws_iam_role.scheduler.arn
    input    = jsonencode({ reason = "max_runtime_hours", hours = var.max_runtime_hours })
  }
}

resource "aws_lambda_permission" "scheduler_invoke" {
  statement_id  = "AllowEventBridgeScheduler"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.kill.function_name
  principal     = "scheduler.amazonaws.com"
  source_arn    = aws_scheduler_schedule.kill_fleet.arn
}
