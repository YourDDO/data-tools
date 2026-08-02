# YourDDO Data Tools v2

This Go 1.26 module is the new home of the complete YourDDO data pipeline. It fetches source pages from the DDO Compendium MediaWiki `api.php`, converts them into canonical master datasets, generates domain-specific data, validates and hashes every output, and prepares immutable releases for publication.

The module currently lives under `v2/` so the existing Amplify deployment tools remain unchanged. It will move to the repository root after the staged migration is accepted.

## Prerequisites

- Go 1.26
- [Task](https://taskfile.dev/) 3.x for the standard development commands
- Network access to a Compendium `api.php` endpoint when fetching data

The Compendium endpoint is deliberately unauthenticated. Production can set `COMPENDIUM_BASE_URL` to an HTTP private-IP URL inside the shared VPC; local development defaults to the public Compendium host.

## Configuration contract

Deployment settings are read from environment variables and validated before the complete pipeline starts. Validation errors identify the variable and expected shape without echoing its value. URLs with embedded credentials are rejected so credentials cannot leak through configuration errors or ordinary startup logs.

| Variable | Local default | Validation and use |
| --- | --- | --- |
| `COMPENDIUM_BASE_URL` | `https://ddocompendium.com` | Absolute HTTP(S) URL with no credentials, query, or fragment. |
| `COMPENDIUM_API_PATH` | `/api.php` | Absolute URL path with no query or fragment. |
| `OUTPUT_DIR` | `build/output` | Non-empty pipeline output path. |
| `AWS_REGION` | None | Required when publication is enabled; must be a valid AWS region name. |
| `DATA_BUCKET` | None | Required when publication is enabled; must be a valid S3 bucket name. |
| `CDN_BASE_URL` | None | Required when publication is enabled; must be an absolute HTTPS URL. |
| `GAME_VERSION` | None | Required numeric `major.minor.patch` DDO version, such as `81.3.0`. |
| `PUBLISH_ENABLED` | `false` | Go boolean controlling publication in the complete pipeline. Production candidate publication uses the explicit S3 command below. |

No region, bucket, CDN endpoint, or game version is guessed. Those values identify a deployment or a release and therefore have no safe local default.

## Dataset and release contracts

Stable JSON structures live in `internal/contracts`. JSON field names are lower camel case and are protected by exact serialization tests. A release identity has two parts:

```json
{
  "gameVersion": "81.3.0",
  "dataVersion": 1785175200
}
```

- `gameVersion` is the current DDO game version.
- `dataVersion` is a Unix timestamp assigned by the publisher only when it publishes a changed candidate snapshot.
- Candidate metadata contains hashes and file metadata but no `dataVersion` or generation time, so repeated generation from identical inputs is deterministic.

`manifest.json` contains `schemaVersion`, the release identity, `masterDatasetSha256`, sorted domain summaries, and sorted generated-file entries. Each file entry identifies its domain, relative path, byte size, and SHA-256 hash. Domain summaries include their name, file count, total bytes, and a deterministic SHA-256 hash. Pipeline and per-stage result contracts contain statuses and messages but no clock-derived duration or generation timestamp.

Published objects use this layout:

```text
/latest.json
/releases/{gameVersion}/{dataVersion}/manifest.json
/releases/{gameVersion}/{dataVersion}/master/
/releases/{gameVersion}/{dataVersion}/{domain}/
/releases/{gameVersion}/{dataVersion}/essence-crafting/
```

`latest.json` is updated last and has this shape:

```json
{
  "gameVersion": "81.3.0",
  "dataVersion": 1785175200,
  "baseUrl": "/releases/81.3.0/1785175200"
}
```

## Directory layout

| Path | Purpose |
| --- | --- |
| `cmd/` | Thin executable entry points for each independently runnable pipeline stage. |
| `internal/compendium/` | Configurable MediaWiki client, source-specific cleanup, and wikitext parsing. |
| `internal/config/` | Shared environment variables, defaults, and configuration validation. |
| `internal/dataset/` | Canonical JSON contracts plus deterministic reading and writing. |
| `internal/domain/` | Pure, canonical-input-only generators and their single registration point. |
| `internal/hashing/` | Stable SHA-256 file and directory hashing. |
| `internal/manifest/` | Candidate release manifests and the production-pointer contract. |
| `internal/pipeline/` | Fetch, change detection, generation, validation, and candidate orchestration. |
| `internal/publish/` | Backend-neutral publication and the development filesystem backend. |
| `internal/validation/` | Master, domain, JSON, and release-manifest validation. |
| `testdata/` | Small, non-production fixtures used by tests and local examples. |
| `scripts/` | Reserved for narrowly scoped automation that does not belong in Go or Task. |
| `infrastructure/` | Terraform bootstrap, production composition, and reusable AWS modules. |
| `Taskfile.yml` | Formatting, linting, testing, building, pipeline, and local-publication tasks. |
| `buildspec.yml` | CodeBuild verification, generation, validation, and conditional S3 publication. |

Generated production datasets are never committed. Local work defaults to `build/output/`, which is ignored.

## Commands

Run all verification from this directory:

```bash
task fmt
task lint
task test
task test-race
task build
task generate-fixtures
task validate-fixtures
task verify
```

The equivalent acceptance commands are:

```bash
GOCACHE=/tmp/yourddo-data-tools-v2-go-build go test ./...
GOCACHE=/tmp/yourddo-data-tools-v2-go-build go build ./cmd/...
```

Fetch a master dataset:

```bash
go run ./cmd/fetch-master \
  --base-url=http://10.0.0.10 \
  --api-path=/api.php \
  --categories=All \
  --output=build/output/master
```

`All` is the default category selection. It fetches every configured concrete
item category, Augments, Filigrees, and Filigree Sets in one run. Each source
category is queried separately and receives its own JSON file.

The master generator parses every returned source record strictly, normalizes
set-like collections, writes through a streaming JSON encoder and SHA-256
hasher, and logs the resulting master dataset hash. It builds in a temporary
sibling directory and atomically replaces the local working dataset only after
generation succeeds. Existing local files are carried forward, generated
category files are overwritten, and `master-index.json` is merged so separate
category runs accumulate into one master dataset. Pipeline and release
generation retain create-only semantics for immutable snapshots.

The generator returns `dataset.Master`, the same typed canonical contract
accepted by every domain generator. The standalone domain command reconstructs
that exact contract with `dataset.LoadMaster`; it does not use a parallel input
schema.

Generate all domain datasets from a master dataset:

```bash
go run ./cmd/generate-domains \
  --master=build/output/master \
  --output=build/output
```

The default selection is `gear-planner`, `zhentarim-attuned`,
`nearly-complete`, `fountain-of-necrotic-might`, `stormreaver-monument`,
`nearly-finished`, `almost-there`, `finishing-touch`, `alchemical`,
`catalyst-crafting`, `trace-of-madness`, `suppressed-power`, `lost-purpose`,
`attuned-by-heroism`, and `dinosaur-bone`. Pass `--domains` to generate a
subset. The command reads only files declared by the canonical
`master-index.json`; it has no source client and performs no network calls.

To explicitly select every registered domain, use:

```bash
go run ./cmd/generate-domains \
  --master=build/output/master \
  --output=build/output \
  --domains=all
```

## Domain output contracts

Gear Planner receives byte-identical copies of every indexed master record
file and `master-index.json`, because it consumes the complete canonical
schemas. Its additional `setBonusIndex.json` contains only item `name` and
`minLevel`, the two fields needed to display and level-sort set members.
Filigree names come from their canonical page titles because the Compendium's
filigree template `name` fields are not reliably unique and can otherwise
cause distinct common or rare records to be deduplicated. Other items retain
their display names unless multiple records in the same set share one; those
variants use their canonical page titles so every set member remains unique.

Fountain of Necrotic Might, Stormreaver Monument, and Zhentarim Attuned emit
`upgrades.json`. Each entry includes the item `name` to identify the upgrade,
`effectsRemoved` to describe the base effects being replaced, and
`effectsAdded` to describe the resulting effects. Upgrade metadata markers are
removed from those effect lists.

The remaining domains emit `items.json` with this intentionally narrow schema:

| Field | Why it is included |
| --- | --- |
| `pageTitle` | Stable canonical record identity, including variant suffixes. |
| `name` | Player-facing item label. |
| `type` | Equipment slot or weapon type used by crafting clients. |
| `minLevel` | Level gating and display. |
| `enchantments` | The qualifying crafting marker and effects presented by the domain. |
| `augments` | Dinosaur Bone items only; these identify their eligible crafting slots. |

Prices, durability, images, source history, and drop details are deliberately
excluded.

Dinosaur Bone also emits `augments.json`, selected from canonical augment
records whose `augmentType` starts with `Isle of Dread:`. Its compact entries
retain the name, compatible augment type, minimum level, resulting effects or
set bonus, and crafting/acquisition requirements. Binding, weight, and update
history are omitted because they are not used by this crafting domain.

| Domain | Canonical selection rule |
| --- | --- |
| `nearly-complete` | Enchantment is `Nearly Complete` or starts with `Nearly Complete: `. |
| `nearly-finished` | Has `Nearly Finished`. |
| `almost-there` | Has `Almost There`. |
| `finishing-touch` | Has `Finishing Touch`. |
| `alchemical` | Has `Alchemical (Prototype)`. |
| `trace-of-madness` | Has `Trace of Madness`. |
| `suppressed-power` | Has `Suppressed Power`. |
| `lost-purpose` | Has `Lost Purpose`. |
| `attuned-by-heroism` | Has a numeric `Attuned by Heroism: Tier N` marker; the Compendium's canonical `Attuned to Heroism: Tier N` spelling is also accepted. |
| `catalyst-crafting` | Union of Trace of Madness, Suppressed Power, and Lost Purpose records. |
| `dinosaur-bone` | Has an `Isle of Dread: ... Slot (...)` canonical augment slot; this covers weapons, armor, jewelry, and accessories. |

All lists use natural name ordering with page title as the tie-breaker. The
registry in `internal/domain/registry` is the only orchestration list: adding a
domain requires implementing `domain.Generator` and adding one registration.
Every generator returns the path, byte size, and SHA-256 of each file it wrote.

Run the complete local pipeline without publication:

```bash
task pipeline \
  GAME_VERSION=81.3.0 \
  COMPENDIUM_BASE_URL=http://10.0.0.10 \
  ESSENCE_INPUT=/path/to/essence-crafting.json \
  FOUNTAIN_INPUT=/path/to/fountain-seed.json
```

`cmd/pipeline` is the primary executable. It creates an isolated work
directory, fetches and normalizes the master dataset, hashes it, compares that
hash with the active local release when one is configured, generates domains
only for changed data, validates and locally assembles the immutable release,
then cleans the work directory. Its JSON result reports one of `published`,
`no-change`, `failed`, or `dry-run`; structured JSON logs are written to
stderr. `--dry-run` performs generation, validation, and local assembly without
publication writes. Use `--debug-preserve` to retain the isolated directory.

Publish and activate to a local filesystem explicitly:

```bash
go run ./cmd/pipeline \
  --game-version=81.3.0 \
  --publish \
  --publish-root=/tmp/yourddo-published
```

The local publisher reads `latest.json` and its manifest for change detection.
All immutable release files and `manifest.json` are uploaded before the
separate activation stage replaces `latest.json`. A validation or upload
failure therefore cannot move the active pointer. Validation checks JSON and
record contracts, identifiers, references, bounds, manifest and hash
agreement, release-file hygiene, and empty outputs. Every failure identifies
its pipeline stage and build context; warnings can be made blocking with
`VALIDATION_WARNINGS_AS_ERRORS=true`.

Publish that candidate to a local filesystem for testing:

```bash
task publish:local PUBLISH_ROOT=/tmp/yourddo-published
```

Objects are written beneath `releases/<game-version>/<data-version>/` with create-only semantics. The publisher assigns `dataVersion` at that point; `latest.json` is replaced only after every data file and the release manifest succeed.

Publish a validated candidate to S3 in production:

```bash
AWS_REGION=us-east-2 \
DATA_BUCKET=yourddo-data-prod \
task publish:s3
```

The S3 backend uses the AWS SDK for Go v2 default credential chain and retryer.
It requires only `s3:PutObject` for the configured bucket keys. Immutable
objects are sent with `If-None-Match: *`, so an existing snapshot cannot be
replaced without a separate read operation or a check-then-write race. No ACL
is set and the bucket remains private. Release objects use
`public, max-age=31536000, immutable`; `latest.json` uses `no-cache`. All
objects are JSON and no content-encoding is declared for the uncompressed
bytes currently published.

## Migration map

| Legacy location | v2 owner |
| --- | --- |
| `dataSpider/api` | `internal/compendium` and `internal/dataset` |
| `dataSpider/cleanup`, `dataSpider/parser` | `internal/compendium/cleanup`, `internal/compendium/parser` |
| `dataSpider/indexer` | `internal/domain/gearplanner` |
| `cannithSplit` | `internal/domain/essencecrafting` |
| `fountainUpdate` | `internal/domain/fountain` |
| `zhentarimUpdate` | `internal/domain/zhentarim` |

“Cannith Crafting” product terminology is emitted as “Essence Crafting” in v2. MediaWiki template names and in-game House Cannith proper nouns remain unchanged because they are source data.

## Production infrastructure

Production publication must use the `s3` backend and is never allowed to fall back to local storage. Terraform under `infrastructure/` defines the private data bucket, CDN, and manually started CodeBuild project while referencing the existing shared network, DNS, certificate, and Compendium resources. See `infrastructure/README.md` for the state bootstrap, ownership boundaries, initialization, and validation workflow. Terraform is never applied automatically.
