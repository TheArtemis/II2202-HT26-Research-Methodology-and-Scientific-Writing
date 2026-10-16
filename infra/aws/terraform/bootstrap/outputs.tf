output "aws_region" {
  description = "Region where bootstrap resources were created."
  value       = var.aws_region
}

output "account_id" {
  description = "AWS account ID used for naming."
  value       = local.account_id
}

output "results_bucket_name" {
  description = "S3 bucket for Terraform state (tfstate/) and campaign results (campaigns/)."
  value       = aws_s3_bucket.experiments.bucket
}

output "results_bucket_arn" {
  description = "ARN of the experiments results bucket."
  value       = aws_s3_bucket.experiments.arn
}

output "dynamodb_lock_table_name" {
  description = "DynamoDB table for Terraform state locking."
  value       = aws_dynamodb_table.terraform_locks.name
}

output "backend_hcl_snippet" {
  description = "Paste into fleet/backend.hcl after bootstrap (or copy backend.hcl.example)."
  value       = <<-EOT
    bucket         = "${aws_s3_bucket.experiments.bucket}"
    key            = "tfstate/fleet/terraform.tfstate"
    region         = "${var.aws_region}"
    dynamodb_table = "${aws_dynamodb_table.terraform_locks.name}"
    encrypt        = true
  EOT
}
