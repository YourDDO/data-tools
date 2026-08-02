# Repository guidance

## Project layout

This repository is a single Go 1.26 module containing the YourDDO data pipeline.

- `cmd/` contains the independently runnable pipeline stages.
- `internal/compendium/` contains the MediaWiki client, cleanup helpers, and parsing logic.
- `internal/domain/` contains deterministic domain generators.
- `internal/pipeline/`, `internal/publish/`, and `internal/validation/` orchestrate, publish, and validate releases.
- `inputs/manual/` contains manually maintained production payloads; `testdata/` contains test fixtures.
- `infrastructure/` contains the Terraform bootstrap, production stack, and reusable AWS modules.

## Build and test commands

Use a build cache under `/tmp`; the default Go cache may be read-only in agent environments.

```bash
GOCACHE=/tmp/yourddo-data-tools-go-build go test ./...
GOCACHE=/tmp/yourddo-data-tools-go-build go vet ./...
GOCACHE=/tmp/yourddo-data-tools-go-build go build ./cmd/...
```

Run the complete quality gate with `task verify`. For a focused parser test:

```bash
GOCACHE=/tmp/yourddo-data-tools-go-build go test ./internal/compendium/parser -run 'TestName'
```

Format only touched Go files with `gofmt -w <files>`. Check Terraform with:

```bash
terraform fmt -check -recursive infrastructure
terraform -chdir=infrastructure/bootstrap init -backend=false
terraform -chdir=infrastructure/bootstrap validate
terraform -chdir=infrastructure/environments/production init -backend=false
terraform -chdir=infrastructure/environments/production validate
```

## Implementation conventions

- Treat JSON structures in `internal/contracts/`, `internal/dataset/`, and domain output types as external data contracts.
- Add parser behavior to the appropriate `internal/compendium/parser/enchantments_*.go` file with table-driven tests in the matching test file.
- Preserve deterministic output: sort map-derived collections, deduplicate index entries, retain established ordering, and terminate generated JSON files with a newline.
- Return or log errors with enough context to identify the stage, category, page, input, or path.
- Prefer standard-library solutions and existing helpers; add dependencies only when they materially simplify the implementation.
- Commit both Terraform lock files when provider selections intentionally change. Never commit local `.terraform/`, `terraform.tfvars`, `backend.hcl`, state, or saved plans.

## Data-generation safety

Do not run network-backed or data-writing commands merely to verify compilation.

- `go run ./cmd/fetch-master` and the complete pipeline call the configured Compendium endpoint and write generated datasets.
- `go run ./cmd/pipeline` may publish immutable releases and update `latest.json` when publication is enabled.
- `go run ./cmd/publish-release` writes to the selected local or S3 publication backend.
- Golden-file regeneration writes fixtures and must only be run when the task explicitly requires it.
- Terraform `plan` and `apply` may access production state or infrastructure; validation should use `init -backend=false` unless remote-state access is explicitly required.

## Before finishing

Run focused tests for changed behavior and `task verify` for the module. Run Terraform formatting and validation for infrastructure changes. Report commands that could not run, distinguish pre-existing failures, and avoid changing unrelated generated data or IDE metadata.
