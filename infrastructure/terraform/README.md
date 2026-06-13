# Terraform — BTC Pilot Secrets Infrastructure

## Prerequisites

- Terraform >= 1.6
- AWS CLI configured with credentials that can create IAM roles, KMS keys, and Secrets Manager secrets
- AWS region: `ap-south-1` (default; override with `-var aws_region=...`)

## Deployment Steps

### 1. Initialise Terraform

```bash
cd infrastructure/terraform
terraform init
```

### 2. Review the plan

```bash
terraform plan -out=tfplan
```

Review the output. Expected resources:
- 1 KMS key + alias
- 1 IAM role + instance profile + policy
- 6 Secrets Manager secrets
- 1 secret rotation schedule (requires Lambda ARN)

### 3. Apply

```bash
terraform apply tfplan
```

### 4. Populate secret values in AWS Console

After `terraform apply`, the secrets exist but have **no values**. Populate them manually:

```bash
# AngelOne TOTP
aws secretsmanager put-secret-value \
  --secret-id /btcpilot/angelone/totp \
  --secret-string "YOUR_TOTP_SECRET"

# AngelOne API key
aws secretsmanager put-secret-value \
  --secret-id /btcpilot/angelone/api-key \
  --secret-string "YOUR_ANGELONE_API_KEY"

# Delta Exchange API key
aws secretsmanager put-secret-value \
  --secret-id /btcpilot/delta/api-key \
  --secret-string "YOUR_DELTA_API_KEY"

# Delta Exchange API secret
aws secretsmanager put-secret-value \
  --secret-id /btcpilot/delta/api-secret \
  --secret-string "YOUR_DELTA_API_SECRET"

# MongoDB URI
aws secretsmanager put-secret-value \
  --secret-id /btcpilot/mongodb/uri \
  --secret-string "YOUR_MONGODB_URI"

# JWT signing secret
aws secretsmanager put-secret-value \
  --secret-id /btcpilot/auth/jwt-secret \
  --secret-string "YOUR_JWT_SECRET"
```

### 5. Attach IAM instance profile to Lightsail instance

```bash
# Get the profile name from Terraform output
PROFILE=$(terraform output -raw engine_instance_profile)

# Attach to Lightsail (via AWS Console → Lightsail → Instance → IAM role)
# or via EC2 API if using EC2 directly:
aws ec2 associate-iam-instance-profile \
  --instance-id i-XXXXXXXX \
  --iam-instance-profile Name=$PROFILE
```

### 6. Verify the engine can read secrets

SSH into the Lightsail instance and test:

```bash
aws secretsmanager get-secret-value \
  --secret-id /btcpilot/angelone/totp \
  --region ap-south-1 \
  --query SecretString \
  --output text
```

If this returns your TOTP secret, the IAM role is correctly attached and the engine will be able to read secrets at startup.

## Secret Rotation

Automatic rotation is configured for `/btcpilot/angelone/totp` (30-day rotation). To enable rotation:

1. Deploy a Lambda function that handles TOTP rotation
2. Pass its ARN to Terraform:
   ```bash
   terraform apply -var totp_rotation_lambda_arn=arn:aws:lambda:ap-south-1:123456789:function:rotate-totp
   ```

## Environment Variable Fallback (Development)

When `USE_LOCAL_SECRETS=true` is set, the engine falls back to environment
variables instead of AWS Secrets Manager. This is safe for local development
but **must not be used in production**.

| Secret path | Env var fallback |
|-------------|-----------------|
| `/btcpilot/angelone/totp` | `ANGELONE_TOTP_SECRET` |
| `/btcpilot/angelone/api-key` | `ANGELONE_API_KEY` |
| `/btcpilot/delta/api-key` | `DELTA_API_KEY` |
| `/btcpilot/delta/api-secret` | `DELTA_API_SECRET` |
| `/btcpilot/mongodb/uri` | `MONGODB_URI` |
| `/btcpilot/auth/jwt-secret` | `AUTH_JWT_SECRET` |
