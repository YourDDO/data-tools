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
| `environments/production/` | The data bucket, CloudFront distribution, `cdn.yourddo.com` certificate and DNS records, CodeBuild project, EventBridge schedule, service roles, security group, log group, one NAT Gateway and Elastic IP, the selected private subnet's default route, and the CodeBuild-to-Compendium ingress rule | The state bucket, shared VPC, Internet Gateway, public and private subnets, route tables, Route 53 hosted zone, source connection, VPC endpoints, or other Compendium infrastructure |

The production stack uses AWS data sources to verify the supplied shared
resource identifiers. It does not declare or import those resources. It owns
only the TCP 80 ingress rule from the CodeBuild security group on the shared
Compendium security group; no other Compendium rules or resources are managed
here.

The existing CodeConnections connection is supplied by ARN and is not owned
by this stack. No personal access token or repository secret is stored in
Terraform. Production builds may be started manually or by the stack's
EventBridge Scheduler schedule. The schedule is disabled by default so an
initial apply cannot publish before a manual production build is verified.

## CodeBuild pipeline

The stack creates one project named `yourddo-data-tools`. It checks out the
configured `main` branch through AWS CodeConnections and uses
`buildspec.yml` from the repository root. A clone depth of one retains the built
commit identity needed by CodeBuild without fetching unrelated history. The
buildspec installs a pinned Task CLI, runs `task verify`, fetches and generates
the live dataset, validates the candidate, and conditionally invokes the S3
publisher. The publisher writes every immutable object and `manifest.json`
under `releases/*` before it updates `latest.json`; any earlier failure leaves
the active pointer unchanged. CodeBuild artifacts and S3 build logs are
disabled because datasets are published directly by the application.

The project uses `BUILD_GENERAL1_SMALL`, `ARM_CONTAINER`, and the AWS-managed
Amazon Linux 2023 AArch64 standard 3.0 image with privileged mode disabled.
The repository's Go code and AWS SDK are architecture-independent, and the
buildspec explicitly selects the image's Go 1.26 runtime instead of relying on
its older default. Task is installed from its pinned Go module during the
install phase. There is therefore no x86-only toolchain requirement to justify
the more expensive architecture.

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

CodeBuild attaches only the dedicated egress-only security group to one
existing private subnet. It retains TCP 443 egress to `0.0.0.0/0` for GitHub,
Go downloads, and AWS APIs, and TCP 80 egress to only the Compendium security
group. The matching Compendium ingress rule allows TCP 80 only from the
CodeBuild security group. Neither service receives an inbound internet rule or
a public IP, and `COMPENDIUM_BASE_URL` remains the Compendium's private IP.

The stack creates one public NAT Gateway and one Elastic IP so CodeBuild can
initiate internet HTTPS connections. The NAT Gateway is placed in the supplied
existing public subnet only after Terraform confirms that subnet's existing
route table sends `0.0.0.0/0` to the VPC Internet Gateway. CodeBuild uses an
existing private subnet in the same Availability Zone when one is available;
that subnet's existing route table receives a `0.0.0.0/0` route to the NAT
Gateway. Private VPC routes still send Compendium traffic directly to its
private IP, so those requests do not traverse the NAT Gateway.

In `us-east-2`, expect roughly $36.50 per 730-hour month in fixed charges for
one NAT Gateway ($0.045/hour) and its public IPv4 address ($0.005/hour), plus
$0.045 per GB processed by the NAT Gateway and normal data-transfer charges.
AWS pricing can change; confirm current regional pricing before applying. An
S3 gateway endpoint can reduce NAT traffic, but public Git hosting and Go
module/toolchain downloads still require internet egress.

### Production inputs and prerequisites

Set these production variables in `terraform.tfvars`:

- owned resource configuration: `data_bucket_name`, `cdn_domain_name`, and
  `game_version`;
- shared infrastructure: `aws_region`, `vpc_id`, `public_subnet_id`,
  `private_subnet_ids`,
  `route53_zone_id`, `compendium_security_group_id`,
  `compendium_base_url`, `compendium_api_path`, and `compendium_port`; and
- source integration: `repository_url`, `repository_connection_arn`,
  `repository_type`, and `source_version`; and
- scheduling: `schedule_expression`, `schedule_timezone`, and
  `schedule_enabled`.

Before applying, the AWS CodeConnections connection must exist in the same
region, be in `AVAILABLE` status, and be authorized for the repository. The
selected public subnet must already have a default route to the VPC Internet
Gateway; Terraform verifies that route before creating the NAT Gateway.
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

EventBridge Scheduler starts the same CodeBuild project and overrides only
`TRIGGER_SOURCE=scheduled`; it does not receive AWS credentials, bucket names,
or other application settings. Configure the cadence with an EventBridge
Scheduler `cron(...)` or `rate(...)` expression and an IANA timezone. After a
manual production build succeeds, set `schedule_enabled = true`, review the
Terraform plan, and apply it. EventBridge Scheduler assumes a dedicated role
that can call `codebuild:StartBuild` only on this CodeBuild project.

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

From the repository root, check every Terraform file with:

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
