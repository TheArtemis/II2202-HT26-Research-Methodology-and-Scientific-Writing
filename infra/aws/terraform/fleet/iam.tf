data "aws_iam_policy_document" "ec2_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "worker" {
  name               = "${var.project}-${var.campaign_id}-worker"
  assume_role_policy = data.aws_iam_policy_document.ec2_assume.json
}

data "aws_iam_policy_document" "worker_s3" {
  statement {
    sid    = "ListCampaignPrefix"
    effect = "Allow"
    actions = [
      "s3:ListBucket",
    ]
    resources = [
      "arn:aws:s3:::${local.results_bucket}",
    ]
    condition {
      test     = "StringLike"
      variable = "s3:prefix"
      values = [
        "campaigns/*",
        "campaigns/",
      ]
    }
  }

  statement {
    sid    = "ObjectRWUnderCampaigns"
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:AbortMultipartUpload",
      "s3:ListMultipartUploadParts",
    ]
    resources = [
      "arn:aws:s3:::${local.results_bucket}/campaigns/*",
    ]
  }
}

resource "aws_iam_role_policy" "worker_s3" {
  name   = "s3-campaigns-only"
  role   = aws_iam_role.worker.id
  policy = data.aws_iam_policy_document.worker_s3.json
}

resource "aws_iam_instance_profile" "worker" {
  name = "${var.project}-${var.campaign_id}-worker"
  role = aws_iam_role.worker.name
}
