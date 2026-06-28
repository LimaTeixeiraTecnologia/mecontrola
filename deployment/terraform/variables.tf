variable "aws_region" {
  description = "AWS region for the backup bucket and IAM resources."
  type        = string
  default     = "us-east-1"
}

variable "project" {
  description = "Project slug used in resource naming."
  type        = string
  default     = "mecontrola"
}

variable "environment" {
  description = "Environment name represented by this stack."
  type        = string
  default     = "staging"
}

variable "backup_bucket_name" {
  description = "Explicit S3 bucket name. Leave null to derive from project and account id."
  type        = string
  default     = null
}

variable "staging_iam_user_name" {
  description = "IAM user name for staging backups. Leave null to derive from project and environment."
  type        = string
  default     = null
}

variable "lifecycle_transition_days" {
  description = "Days before S3 objects transition to Glacier Flexible Retrieval."
  type        = number
  default     = 90
}

variable "tags" {
  description = "Additional tags applied to managed resources."
  type        = map(string)
  default     = {}
}
