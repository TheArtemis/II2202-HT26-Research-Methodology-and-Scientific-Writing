terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.4"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.11"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.4"
    }
  }

  # Configure after bootstrap:
  #   terraform init -backend-config=backend.hcl
  backend "s3" {}
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = var.project
      Campaign  = var.campaign_id
      ManagedBy = "terraform"
      Component = "fleet"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

data "aws_ami" "ubuntu_2204" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }
}

locals {
  account_id     = data.aws_caller_identity.current.account_id
  results_bucket = coalesce(var.results_bucket_name, "ii2202-experiments-${local.account_id}")
  subnet_ids     = data.aws_subnets.default.ids
  worker_indexes = range(var.worker_count)
  worker_names   = [for i in local.worker_indexes : format("worker-%02d", i)]
  # Equal shard size for the locked 900-trial campaign (matches ops TOTAL_TRIALS default).
  total_trials        = 900
  trials_per_worker   = ceil(local.total_trials / var.worker_count)
}
