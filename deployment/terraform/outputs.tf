output "backup_bucket_name" {
  description = "Backup bucket name for .env and pgBackRest."
  value       = aws_s3_bucket.backups.bucket
}

output "backup_bucket_arn" {
  description = "Backup bucket ARN."
  value       = aws_s3_bucket.backups.arn
}

output "backup_region" {
  description = "AWS region used by the backup bucket."
  value       = var.aws_region
}

output "staging_iam_user_name" {
  description = "IAM user created for staging backup operations."
  value       = aws_iam_user.staging_backups.name
}

output "staging_access_key_id" {
  description = "Access key id for the staging backup IAM user."
  value       = aws_iam_access_key.staging_backups.id
}

output "staging_secret_access_key" {
  description = "Secret access key for the staging backup IAM user."
  value       = aws_iam_access_key.staging_backups.secret
  sensitive   = true
}

output "staging_env" {
  description = "Non-sensitive .env values for staging."
  value = {
    PGBACKREST_S3_BUCKET   = aws_s3_bucket.backups.bucket
    PGBACKREST_S3_ENDPOINT = "s3.${var.aws_region}.amazonaws.com"
    PGBACKREST_S3_REGION   = var.aws_region
  }
}

output "staging_sensitive_env" {
  description = "Sensitive .env values for staging."
  value = {
    AWS_ACCESS_KEY_ID        = aws_iam_access_key.staging_backups.id
    AWS_SECRET_ACCESS_KEY    = aws_iam_access_key.staging_backups.secret
    PGBACKREST_S3_KEY        = aws_iam_access_key.staging_backups.id
    PGBACKREST_S3_KEY_SECRET = aws_iam_access_key.staging_backups.secret
  }
  sensitive = true
}
