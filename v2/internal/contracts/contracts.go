// Package contracts defines the stable JSON documents exchanged by pipeline
// stages and consumed by dataset clients.
package contracts

const ReleaseManifestSchemaVersion = 1

type PipelineOutcome string

const (
	PipelineOutcomePublished PipelineOutcome = "published"
	PipelineOutcomeNoChange  PipelineOutcome = "no-change"
	PipelineOutcomeFailed    PipelineOutcome = "failed"
	PipelineOutcomeDryRun    PipelineOutcome = "dry-run"
)

// ReleaseIdentity uniquely identifies one immutable published snapshot.
type ReleaseIdentity struct {
	GameVersion string `json:"gameVersion"`
	DataVersion int64  `json:"dataVersion"`
}

// DatasetMetadata summarizes all generated files belonging to one domain.
type DatasetMetadata struct {
	Domain    string `json:"domain"`
	FileCount int    `json:"fileCount"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

// GeneratedFileMetadata describes one immutable file in a release.
type GeneratedFileMetadata struct {
	Domain    string `json:"domain"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

// ReleaseManifest is written to manifest.json in an immutable release.
// It deliberately contains no generation timestamp.
type ReleaseManifest struct {
	SchemaVersion int `json:"schemaVersion"`
	ReleaseIdentity
	MasterDatasetSHA256 string                  `json:"masterDatasetSha256"`
	Domains             []DatasetMetadata       `json:"domains"`
	GeneratedFiles      []GeneratedFileMetadata `json:"generatedFiles"`
}

// Latest is the stable /latest.json pointer to the current release.
type Latest struct {
	ReleaseIdentity
	BaseURL string `json:"baseUrl"`
}

// PipelineStageResult reports one stage without adding nondeterministic timing
// information to pipeline output.
type PipelineStageResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Changed bool   `json:"changed,omitempty"`
	Message string `json:"message,omitempty"`
}

// PipelineResult is the machine-readable result of a pipeline invocation.
// Release is present after a changed snapshot has been assembled, including
// dry runs; Published distinguishes activation from local assembly.
type PipelineResult struct {
	Outcome   PipelineOutcome       `json:"outcome"`
	Changed   bool                  `json:"changed"`
	Published bool                  `json:"published"`
	OutputDir string                `json:"outputDir"`
	Release   *ReleaseIdentity      `json:"release,omitempty"`
	Stages    []PipelineStageResult `json:"stages"`
	Warnings  []string              `json:"warnings,omitempty"`
}
