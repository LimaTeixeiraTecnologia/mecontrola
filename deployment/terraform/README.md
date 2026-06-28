# Terraform AWS Bootstrap for Backups

This stack provisions the AWS resources required to unblock `.specs/prd-infra-producao-robusta-10k-dez-2026` tasks `6.0` and `8.0`:

- one S3 backup bucket in `us-east-1`
- one IAM user restricted to `pgbackrest/` and `mecontrola-env-backups/`
- one access key for the staging environment

The Terraform project lives in `deployment/terraform/` and uses an S3 backend with lockfile support, as recommended in the official Terraform backend documentation.

## Prerequisites

- Terraform installed locally
- AWS CLI installed locally
- an AWS admin profile already configured
- a dedicated S3 bucket for Terraform state created manually before the first `terraform init`

The backend state bucket must be separate from the application backup bucket to avoid circular bootstrap and mixed responsibilities.

## Files

- `versions.tf`: Terraform and provider constraints
- `backend.tf`: partial `s3` backend configuration
- `provider.tf`: AWS provider configuration
- `main.tf`: S3 bucket and IAM resources
- `variables.tf`: configurable inputs
- `outputs.tf`: values needed by staging `.env`
- `terraform.tfvars.example`: example variables for local use

## 1. Configure AWS locally

Use an admin-capable profile before applying:

```bash
aws configure --profile mecontrola-bootstrap-admin
aws sts get-caller-identity --profile mecontrola-bootstrap-admin
```

Export the profile for Terraform:

```bash
export AWS_PROFILE=mecontrola-bootstrap-admin
```

## 2. Create the Terraform state bucket manually

This bucket is not managed by this stack. Create it once, then reuse it for subsequent applies.

Suggested name:

```text
mecontrola-terraform-state-<account-id>-use1
```

Suggested AWS CLI commands:

```bash
aws s3api create-bucket \
  --bucket mecontrola-terraform-state-123456789012-use1 \
  --region us-east-1

aws s3api put-bucket-versioning \
  --bucket mecontrola-terraform-state-123456789012-use1 \
  --versioning-configuration Status=Enabled

aws s3api put-bucket-encryption \
  --bucket mecontrola-terraform-state-123456789012-use1 \
  --server-side-encryption-configuration '{
    "Rules": [
      {
        "ApplyServerSideEncryptionByDefault": {
          "SSEAlgorithm": "AES256"
        }
      }
    ]
  }'
```

## 3. Create a local tfvars file

```bash
cp terraform.tfvars.example terraform.tfvars
```

Adjust values only if you need to override defaults.

## 4. Initialize Terraform

Run from `deployment/terraform/`:

```bash
terraform init \
  -backend-config="bucket=mecontrola-terraform-state-123456789012-use1" \
  -backend-config="key=mecontrola/deployment/terraform.tfstate" \
  -backend-config="region=us-east-1"
```

## 5. Review and apply

```bash
terraform plan
terraform apply
```

## 6. Export values for staging

Non-sensitive outputs:

```bash
terraform output staging_env
```

Sensitive outputs:

```bash
terraform output -raw staging_access_key_id
terraform output -raw staging_secret_access_key
terraform output -json staging_sensitive_env
```

Fill the staging `.env` with:

```text
AWS_ACCESS_KEY_ID=<staging_access_key_id>
AWS_SECRET_ACCESS_KEY=<staging_secret_access_key>
PGBACKREST_S3_BUCKET=<backup_bucket_name>
PGBACKREST_S3_REGION=us-east-1
PGBACKREST_S3_ENDPOINT=s3.us-east-1.amazonaws.com
PGBACKREST_S3_KEY=<staging_access_key_id>
PGBACKREST_S3_KEY_SECRET=<staging_secret_access_key>
```

## 7. Validate integration with the repo

From the repository root:

```bash
bash deployment/scripts/backup-env-s3.sh /path/to/staging/.env
```

Then run the two-phase pgBackRest setup on the staging host:

```bash
sudo bash deployment/scripts/pgbackrest-setup.sh
sudo bash deployment/scripts/pgbackrest-setup.sh --backup
```

After S3 uploads and pgBackRest checks succeed, re-execute task `6.0`. Once `6.0` is green, re-execute task `8.0`.
