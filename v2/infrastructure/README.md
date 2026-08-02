# Terraform infrastructure

This directory contains the Terraform bootstrap and production stack for the
YourDDO data pipeline. Terraform is pinned to `1.15.8` and the AWS provider is
pinned to `6.56.0`. Commit both `.terraform.lock.hcl` files whenever provider
selections change.

Terraform never reads credentials from these files. Authenticate with the
normal AWS credential chain (for example, AWS IAM Identity Center, an assumed
role, or environment variables) before running Terraform.

## Ownership boundaries

The bootstrap and production configurations have separate state and ownership:

| Configuration | Owns | Does not own |
| --- | --- | --- |
| `bootstrap/` | The encrypted, versioned, private S3 state bucket | Production resources or a DynamoDB lock table |
| `environments/production/` | The data bucket, CloudFront distribution, `cdn.yourddo.com` certificate and DNS records, CodeBuild project, service role, security group, and log group | The state bucket, shared VPC, private subnets, Route 53 hosted zone, source connection, NAT gateways, VPC endpoints, or Compendium infrastructure |

The production stack uses AWS data sources to verify the supplied shared
resource identifiers. It does not declare those resources, import them, or
modify the Compendium security group. The shared Compendium stack must allow
its configured TCP port from the CodeBuild security group output by this
stack. Private subnets must already have egress through existing NAT or VPC
endpoints for S3, CloudWatch Logs, CodeConnections, and public HTTPS package
downloads. This stack intentionally creates no NAT gateway or interface
endpoint because those shared, hourly billed resources must be selected at
the VPC level rather than duplicated for one build project.

The existing CodeConnections connection is supplied by ARN and is not owned
by this stack. No personal access token or repository secret is stored in
Terraform. No webhook or EventBridge schedule is attached; production builds
are manual until trigger work is introduced separately.

## CodeBuild pipeline

The stack creates one project named `yourddo-data-tools`. It checks out the
configured `main` branch through AWS CodeConnections and uses
`v2/buildspec.yml` from the repository. A clone depth of one retains the built
commit identity needed by CodeBuild without fetching unrelated history. The
buildspec installs a pinned Task CLI, runs `task verify`, fetches and generates
the live dataset, validates the candidate, and conditionally invokes the S3
publisher. The publisher writes every immutable object and `manifest.json`
under `releases/*` before it updates `latest.json`; any earlier failure leaves
the active pointer unchanged. CodeBuild artifacts and S3 build logs are
disabled because datasets are published directly by the application.

The project uses `BUILD_GENERAL1_SMALL`, `ARM_CONTAINER`, and the AWS-managed
Amazon Linux 2023 AArch64 standard 3.0 image with privileged mode disabled.
The repository's Go code and AWS SDK are architecture-independent, and this
image supplies Bash, Git, AWS CLI, and a Go toolchain capable of honoring the
module's Go 1.26 toolchain directive. Task is installed from its pinned Go
module during the install phase. There is therefore no x86-only toolchain
requirement to justify the more expensive architecture.

Build output is retained for 30 days in the dedicated CloudWatch Logs group
`/aws/codebuild/yourddo-data-tools`, using CloudWatch Logs' default encryption.
The build and queue timeouts are both 30 minutes.

### IAM scope

The dedicated service role can:

- create its configured log group and write streams and events only beneath
  that group;
- list only the `latest.json` and `releases/*` prefixes in the production data
  bucket, and get or put only those objects;
- use only the configured CodeConnections connection;
- read only explicitly configured Parameter Store parameters or Secrets
  Manager secrets; and
- manage the ephemeral VPC network interfaces required by CodeBuild.

The publisher does not call `s3:DeleteObject`, so that permission is omitted.
EC2 create/delete and describe actions remain `Resource = "*"` where the APIs
do not support practical static resource scoping. The separate
`ec2:CreateNetworkInterfacePermission` action is restricted to CodeBuild, this
account and region, and the configured private subnet ARNs.

### Network path

CodeBuild attaches only the dedicated egress-only security group to the
existing private subnets. It can reach the Compendium security group on
`compendium_port`, and can use HTTPS for S3, CloudWatch Logs, repository
access, Go toolchain/Task downloads, and Go modules. The shared Compendium
stack must allow inbound TCP from the output
`codebuild_security_group_id`; this stack deliberately does not take
ownership of rules on that external security group.

The selected private subnets must already provide the corresponding routes.
An existing NAT gateway is sufficient. An S3 gateway endpoint plus interface
endpoints for required AWS APIs can reduce NAT traffic, but public Git hosting
and module/toolchain downloads still require compatible outbound access. This
stack creates no NAT gateway or VPC endpoint and therefore adds no associated
hourly network resource cost.

### Production inputs and prerequisites

Set these production variables in `terraform.tfvars`:

- owned resource configuration: `data_bucket_name`, `cdn_domain_name`, and
  `game_version`;
- shared infrastructure: `aws_region`, `vpc_id`, `private_subnet_ids`,
  `route53_zone_id`, `compendium_security_group_id`,
  `compendium_base_url`, `compendium_api_path`, and `compendium_port`; and
- source integration: `repository_url`, `repository_connection_arn`,
  `repository_type`, and `source_version`.

Before applying, the AWS CodeConnections connection must exist in the same
region, be in `AVAILABLE` status, and be authorized for the repository. The
private subnet routes and Compendium ingress described above must also exist.
Optional SSM or Secrets Manager entries are references to existing resources;
the stack creates no speculative secrets.

Terraform sets `AWS_REGION`, `DATA_BUCKET`, `CDN_BASE_URL`, `GAME_VERSION`,
`PUBLISH_ENABLED`, `OUTPUT_DIR=/tmp/yourddo-data`,
`TRIGGER_SOURCE=manual`, `COMPENDIUM_BASE_URL`, and
`COMPENDIUM_API_PATH` as non-secret build environment variables. To change
the permanent game version, update `game_version` and apply the reviewed plan.
To pause publication while continuing tests and live generation, set
`publish_enabled = false` and apply. For one diagnostic build, override only
`PUBLISH_ENABLED` at `start-build` time instead of changing state.

Start a normal manual build with:

```bash
aws codebuild start-build \
  --project-name yourddo-data-tools
```

Then inspect `/aws/codebuild/yourddo-data-tools` in CloudWatch Logs or use the
`codebuild_log_group_name` Terraform output.

## State bootstrap

Bootstrap starts with local state because the remote state bucket does not yet
exist. Copy the example values, choose a globally unique bucket name, review a
plan, and apply it explicitly:

```bash
cd infrastructure/bootstrap
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform fmt -check
terraform validate
terraform plan -out=bootstrap.tfplan
terraform apply bootstrap.tfplan
```

No command in this repository applies Terraform automatically. Preserve the
bootstrap state securely; it is the ownership record for the state bucket.
The bucket has versioning, server-side encryption, public-access blocking, and
deletion protection. No DynamoDB table is needed because the production S3
backend uses S3 native locking.

## Production initialization

Copy the examples and set identifiers for existing shared infrastructure. The
example files contain placeholders but no secrets:

```bash
cd infrastructure/environments/production
cp backend.hcl.example backend.hcl
cp terraform.tfvars.example terraform.tfvars
terraform init -backend-config=backend.hcl
terraform fmt -check
terraform validate
terraform plan -out=production.tfplan
```

`backend.hcl` points at the bucket created by bootstrap and confirms
`use_lockfile = true`, which enables S3 native state locking. Its state key is
separate from the bootstrap state. Backend bucket, key, and region settings
cannot use Terraform input variables, so they are supplied during
`terraform init` instead of being embedded in the stack.

Do not run `terraform import` for manually created or shared resources. They
remain external dependencies referenced through variables and data sources.

## Formatting, validation, and provider locks

From `v2/`, check every Terraform file with:

```bash
terraform fmt -check -recursive infrastructure
terraform -chdir=infrastructure/bootstrap init -backend=false
terraform -chdir=infrastructure/bootstrap validate
terraform -chdir=infrastructure/environments/production init -backend=false
terraform -chdir=infrastructure/environments/production validate
```

When intentionally updating a provider selection, edit the exact version in
every `versions.tf`, then regenerate and review both lock files:

```bash
terraform -chdir=infrastructure/bootstrap init -backend=false -upgrade
terraform -chdir=infrastructure/environments/production init -backend=false -upgrade
```

Local `terraform.tfvars`, `backend.hcl`, `.terraform/` directories, plans, and
state files are ignored. Example configuration and provider lock files are
committed.
