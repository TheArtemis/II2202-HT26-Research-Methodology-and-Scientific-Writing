resource "aws_s3_bucket" "experiments" {
  bucket = local.bucket_name
  tags   = local.common_tags
}

resource "aws_s3_bucket_versioning" "experiments" {
  bucket = aws_s3_bucket.experiments.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "experiments" {
  bucket = aws_s3_bucket.experiments.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "experiments" {
  bucket = aws_s3_bucket.experiments.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "experiments" {
  bucket = aws_s3_bucket.experiments.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

# Marker prefixes so the layout is visible in the console (zero-byte objects).
resource "aws_s3_object" "tfstate_prefix" {
  bucket       = aws_s3_bucket.experiments.id
  key          = "tfstate/"
  content      = ""
  content_type = "application/x-directory"
}

resource "aws_s3_object" "campaigns_prefix" {
  bucket       = aws_s3_bucket.experiments.id
  key          = "campaigns/"
  content      = ""
  content_type = "application/x-directory"
}

resource "aws_dynamodb_table" "terraform_locks" {
  name         = var.lock_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  tags = local.common_tags
}
