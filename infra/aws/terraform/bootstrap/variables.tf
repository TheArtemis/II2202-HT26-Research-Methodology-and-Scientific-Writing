variable "aws_region" {
  description = "AWS region for the experiment results bucket and lock table."
  type        = string
  default     = "eu-north-1"
}

variable "project" {
  description = "Project tag applied to bootstrap resources."
  type        = string
  default     = "ii2202"
}

variable "lock_table_name" {
  description = "DynamoDB table used for Terraform state locking."
  type        = string
  default     = "ii2202-terraform-locks"
}
