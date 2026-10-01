variable "aws_region" {
  description = "AWS region for the disposable fleet (must match bootstrap)."
  type        = string
  default     = "eu-north-1"
}

variable "project" {
  description = "Project tag."
  type        = string
  default     = "ii2202"
}

variable "campaign_id" {
  description = "Campaign identifier used in tags and S3 paths (e.g. full-20260930)."
  type        = string

  validation {
    condition     = can(regex("^[a-zA-Z0-9][a-zA-Z0-9._-]{0,39}$", var.campaign_id))
    error_message = "campaign_id must be 1-40 chars: alphanumeric, dots, underscores, hyphens."
  }
}

variable "worker_count" {
  description = "Number of identical worker instances."
  type        = number
  default     = 8

  validation {
    condition     = var.worker_count >= 1 && var.worker_count <= 20
    error_message = "worker_count must be between 1 and 20."
  }
}

variable "instance_type" {
  description = "EC2 instance type for each worker."
  type        = string
  default     = "c5a.large"
}

variable "root_volume_gb" {
  description = "Root gp3 volume size in GiB."
  type        = number
  default     = 40
}

variable "ssh_cidr" {
  description = "CIDR allowed to SSH to workers (typically your public IP /32)."
  type        = string

  validation {
    condition     = can(cidrhost(var.ssh_cidr, 0))
    error_message = "ssh_cidr must be a valid CIDR block (e.g. 203.0.113.10/32)."
  }
}

variable "ssh_public_key" {
  description = "SSH public key material for the experiment key pair."
  type        = string
  sensitive   = true
}

variable "results_bucket_name" {
  description = "S3 bucket from bootstrap. Empty = ii2202-experiments-<account_id>."
  type        = string
  default     = ""
}

variable "inventory_path" {
  description = "Where to write the generated Ansible inventory (relative to this module)."
  type        = string
  default     = "../../ansible/inventory.ini"
}

variable "fleet_json_path" {
  description = "Where to write fleet.json (relative to this module)."
  type        = string
  default     = "../../ansible/fleet.json"
}
