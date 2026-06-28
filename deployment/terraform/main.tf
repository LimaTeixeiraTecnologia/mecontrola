data "aws_caller_identity" "current" {}

locals {
  backup_bucket_name = coalesce(
    var.backup_bucket_name,
    format("%s-backups-%s-use1", var.project, data.aws_caller_identity.current.account_id)
  )

  staging_iam_user_name = coalesce(
    var.staging_iam_user_name,
    format("%s-%s-s3", var.project, var.environment)
  )

  bucket_arn            = aws_s3_bucket.backups.arn
  env_backup_prefix_arn = "${local.bucket_arn}/mecontrola-env-backups/*"
  pgbackrest_prefix_arn = "${local.bucket_arn}/pgbackrest/*"
  default_tags = merge(
    {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "terraform"
      Component   = "backup"
    },
    var.tags
  )
}

resource "aws_s3_bucket" "backups" {
  bucket = local.backup_bucket_name
}

resource "aws_s3_bucket_versioning" "backups" {
  bucket = aws_s3_bucket.backups.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "backups" {
  bucket = aws_s3_bucket.backups.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id

  rule {
    id     = "archive-after-90-days"
    status = "Enabled"

    filter {
      prefix = ""
    }

    transition {
      days          = var.lifecycle_transition_days
      storage_class = "GLACIER"
    }
  }
}

resource "aws_iam_user" "staging_backups" {
  name = local.staging_iam_user_name
  path = "/system/"
}

resource "aws_iam_access_key" "staging_backups" {
  user = aws_iam_user.staging_backups.name
}

resource "aws_iam_user_policy" "staging_backups" {
  name = format("%s-backup-access", local.staging_iam_user_name)
  user = aws_iam_user.staging_backups.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowBucketMetadata"
        Effect = "Allow"
        Action = [
          "s3:GetBucketLocation",
          "s3:ListBucket"
        ]
        Resource = local.bucket_arn
      },
      {
        Sid    = "AllowPgBackRestObjects"
        Effect = "Allow"
        Action = [
          "s3:AbortMultipartUpload",
          "s3:DeleteObject",
          "s3:GetObject",
          "s3:ListMultipartUploadParts",
          "s3:PutObject"
        ]
        Resource = local.pgbackrest_prefix_arn
      },
      {
        Sid    = "AllowEnvBackupObjects"
        Effect = "Allow"
        Action = [
          "s3:AbortMultipartUpload",
          "s3:DeleteObject",
          "s3:GetObject",
          "s3:ListMultipartUploadParts",
          "s3:PutObject"
        ]
        Resource = local.env_backup_prefix_arn
      }
    ]
  })
}
