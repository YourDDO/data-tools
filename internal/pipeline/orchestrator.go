package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yourddo-data-tools/internal/compendium"
	"yourddo-data-tools/internal/config"
	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/hashing"
	"yourddo-data-tools/internal/manifest"
	"yourddo-data-tools/internal/manual"
	"yourddo-data-tools/internal/publish"
	"yourddo-data-tools/internal/validation"
)

const (
	StageConfiguration   = "configuration"
	StageFetch           = "fetch"
	StageNormalize       = "normalize"
	StageWriteMaster     = "write-master"
	StageHashMaster      = "hash-master"
	StagePrepareManual   = "prepare-manual"
	StageCompare         = "compare"
	StageGenerateDomains = "generate-domains"
	StageValidate        = "validate"
	StageAssembleRelease = "assemble-release"
	StagePublish         = "publish"
	StageActivateRelease = "activate-release"
)

type ActiveReleaseReader interface {
	ActiveReleaseFingerprint(context.Context) (active publish.ActiveRelease, available bool, err error)
}

type ExecuteOptions struct {
	DryRun       bool
	Publish      bool
	PreserveWork bool
}

type OrchestratorDependencies struct {
	Source    compendium.Source
	Active    ActiveReleaseReader
	Store     publish.ObjectStore
	Clock     func() time.Time
	Logger    *slog.Logger
	MkdirTemp func(string, string) (string, error)
	RemoveAll func(string) error
}

// StageError retains the underlying failure while identifying the pipeline
// stage and deterministic build context. It never includes source URLs.
type StageError struct {
	Stage        string
	GameVersion  string
	MasterSHA256 string
	Err          error
}

func (e *StageError) Error() string {
	context := "game_version=" + e.GameVersion
	if e.MasterSHA256 != "" {
		context += " master_sha256=" + e.MasterSHA256
	}
	return fmt.Sprintf("pipeline stage %s failed (%s): %v", e.Stage, context, e.Err)
}

func (e *StageError) Unwrap() error { return e.Err }

// Execute runs the complete local-first pipeline. All generation happens in
// one isolated directory, which is retained only when PreserveWork is set.
func Execute(ctx context.Context, cfg config.Config, options ExecuteOptions, dependencies OrchestratorDependencies) (result Result, returnErr error) {
	result.PipelineResult.Outcome = contracts.PipelineOutcomeFailed
	result.PipelineResult.Stages = make([]contracts.PipelineStageResult, 0, 12)
	logger := dependencies.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	if dependencies.MkdirTemp == nil {
		dependencies.MkdirTemp = os.MkdirTemp
	}
	if dependencies.RemoveAll == nil {
		dependencies.RemoveAll = os.RemoveAll
	}

	complete := func(stage string, changed bool, message string) {
		result.Stages = append(result.Stages, contracts.PipelineStageResult{Name: stage, Status: "succeeded", Changed: changed, Message: message})
		logger.InfoContext(ctx, "pipeline stage completed", "stage", stage, "game_version", cfg.GameVersion, "master_sha256", result.Candidate.MasterDatasetSHA256)
	}
	fail := func(stage string, err error) error {
		result.Stages = append(result.Stages, contracts.PipelineStageResult{Name: stage, Status: "failed", Message: "stage failed"})
		wrapped := &StageError{Stage: stage, GameVersion: cfg.GameVersion, MasterSHA256: result.Candidate.MasterDatasetSHA256, Err: err}
		logger.ErrorContext(ctx, "pipeline stage failed", "stage", stage, "game_version", cfg.GameVersion, "master_sha256", result.Candidate.MasterDatasetSHA256, "error", wrapped)
		return wrapped
	}
	skip := func(stage, message string) {
		result.Stages = append(result.Stages, contracts.PipelineStageResult{Name: stage, Status: "skipped", Message: message})
		logger.InfoContext(ctx, "pipeline stage skipped", "stage", stage, "game_version", cfg.GameVersion, "master_sha256", result.Candidate.MasterDatasetSHA256, "reason", message)
	}

	logger.InfoContext(ctx, "pipeline stage started", "stage", StageConfiguration, "game_version", cfg.GameVersion)
	validationConfig := cfg
	validationConfig.PublishEnabled = false // Publication backend validation is performed by cmd/pipeline.
	if err := validationConfig.Validate(); err != nil {
		return result, fail(StageConfiguration, err)
	}
	if dependencies.Source == nil {
		return result, fail(StageConfiguration, fmt.Errorf("Compendium source is required"))
	}
	if len(cfg.Categories) == 0 {
		return result, fail(StageConfiguration, fmt.Errorf("at least one Compendium category is required"))
	}
	if len(cfg.Domains) == 0 {
		return result, fail(StageConfiguration, fmt.Errorf("at least one domain is required"))
	}
	if options.Publish && dependencies.Store == nil {
		return result, fail(StageConfiguration, fmt.Errorf("publication store is required when publishing is enabled"))
	}
	if options.Publish && dependencies.Active == nil {
		return result, fail(StageConfiguration, fmt.Errorf("active release reader is required when publishing is enabled"))
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return result, fail(StageConfiguration, fmt.Errorf("create work directory: %w", err))
	}
	result.OutputDir = cfg.OutputDir
	workspace, err := dependencies.MkdirTemp(cfg.OutputDir, ".pipeline-*")
	if err != nil {
		return result, fail(StageConfiguration, fmt.Errorf("create isolated work directory: %w", err))
	}
	if options.PreserveWork {
		result.OutputDir = workspace
	}
	defer func() {
		if options.PreserveWork {
			logger.InfoContext(ctx, "preserved pipeline work directory", "stage", "cleanup", "game_version", cfg.GameVersion, "path", workspace)
			return
		}
		if cleanupErr := dependencies.RemoveAll(workspace); cleanupErr != nil {
			cleanupErr = &StageError{Stage: "cleanup", GameVersion: cfg.GameVersion, MasterSHA256: result.Candidate.MasterDatasetSHA256, Err: cleanupErr}
			result.Outcome = contracts.PipelineOutcomeFailed
			logger.ErrorContext(ctx, "pipeline cleanup failed", "stage", "cleanup", "game_version", cfg.GameVersion, "master_sha256", result.Candidate.MasterDatasetSHA256, "error", cleanupErr)
			returnErr = errors.Join(returnErr, cleanupErr)
		}
	}()
	complete(StageConfiguration, false, "")

	candidateRoot := filepath.Join(workspace, "candidate")
	masterRoot := filepath.Join(candidateRoot, "master")
	logger.InfoContext(ctx, "pipeline stage started", "stage", StageFetch, "game_version", cfg.GameVersion)
	generator, err := compendium.NewGenerator(dependencies.Source)
	if err != nil {
		return result, fail(StageFetch, err)
	}
	masterResult, err := generator.Generate(ctx, cfg.Categories, masterRoot)
	if err != nil {
		return result, fail(masterGenerationFailureStage(err), err)
	}
	complete(StageFetch, true, "")
	complete(StageNormalize, true, "")
	complete(StageWriteMaster, true, "")

	logger.InfoContext(ctx, "pipeline stage started", "stage", StageHashMaster, "game_version", cfg.GameVersion)
	masterHash, err := hashing.Directory(masterRoot)
	if err != nil {
		return result, fail(StageHashMaster, err)
	}
	result.Candidate.MasterDatasetSHA256 = masterHash
	complete(StageHashMaster, true, "")

	logger.InfoContext(ctx, "pipeline stage started", "stage", StagePrepareManual, "game_version", cfg.GameVersion, "master_sha256", masterHash)
	manualPayloads, err := manual.Prepare(cfg.ManualInputDir, filepath.Join(candidateRoot, "manual"))
	if err != nil {
		return result, fail(StagePrepareManual, err)
	}
	coverageWarnings, err := itemSetDefinitionCoverage(masterResult.Master, filepath.Join(candidateRoot, "manual"))
	if err != nil {
		return result, fail(StagePrepareManual, err)
	}
	for _, warning := range coverageWarnings {
		logger.WarnContext(ctx, "item-set definitions are missing from the manual payload",
			"stage", StagePrepareManual,
			"game_version", cfg.GameVersion,
			"master_sha256", masterHash,
			"source", warning.source,
			"manual_payload", filepath.ToSlash(filepath.Join("manual", manual.ItemSetEnchantmentsFile)),
			"manual_payload_found", warning.manualFound,
			"missing_count", len(warning.missingSetNames),
			"missing_sets", warning.missingSetNames,
		)
	}
	releaseFingerprint, err := manifest.ReleaseFingerprint(masterHash, manualPayloads)
	if err != nil {
		return result, fail(StagePrepareManual, err)
	}
	result.Candidate.ReleaseFingerprint = releaseFingerprint
	result.Candidate.ManualPayloads = manualPayloads
	complete(StagePrepareManual, true, "")

	logger.InfoContext(ctx, "pipeline stage started", "stage", StageCompare, "game_version", cfg.GameVersion, "master_sha256", masterHash, "release_fingerprint", releaseFingerprint)
	if dependencies.Active != nil {
		active, available, err := dependencies.Active.ActiveReleaseFingerprint(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "release fingerprint comparison", "stage", StageCompare, "latestObjectKey", active.LatestObjectKey, "activeManifestKey", active.ActiveManifestKey, "activeMasterHash", active.MasterSHA256, "generatedMasterHash", masterHash, "activeReleaseFingerprint", active.ReleaseFingerprint, "generatedReleaseFingerprint", releaseFingerprint, "comparisonResult", "error")
			return result, fail(StageCompare, err)
		}
		if !available {
			logger.InfoContext(ctx, "release fingerprint comparison", "stage", StageCompare, "latestObjectKey", active.LatestObjectKey, "activeManifestKey", active.ActiveManifestKey, "activeMasterHash", active.MasterSHA256, "generatedMasterHash", masterHash, "activeReleaseFingerprint", active.ReleaseFingerprint, "generatedReleaseFingerprint", releaseFingerprint, "comparisonResult", "initial-publication")
		} else if active.ReleaseFingerprint == releaseFingerprint {
			logger.InfoContext(ctx, "release fingerprint comparison", "stage", StageCompare, "latestObjectKey", active.LatestObjectKey, "activeManifestKey", active.ActiveManifestKey, "activeMasterHash", active.MasterSHA256, "generatedMasterHash", masterHash, "activeReleaseFingerprint", active.ReleaseFingerprint, "generatedReleaseFingerprint", releaseFingerprint, "comparisonResult", "no-change")
			complete(StageCompare, false, "active release inputs are unchanged")
			result.Outcome = contracts.PipelineOutcomeNoChange
			result.Changed = false
			return result, nil
		} else {
			logger.InfoContext(ctx, "release fingerprint comparison", "stage", StageCompare, "latestObjectKey", active.LatestObjectKey, "activeManifestKey", active.ActiveManifestKey, "activeMasterHash", active.MasterSHA256, "generatedMasterHash", masterHash, "activeReleaseFingerprint", active.ReleaseFingerprint, "generatedReleaseFingerprint", releaseFingerprint, "comparisonResult", "changed")
		}
	} else {
		logger.InfoContext(ctx, "release fingerprint comparison", "stage", StageCompare, "latestObjectKey", "", "activeManifestKey", "", "activeMasterHash", "", "generatedMasterHash", masterHash, "activeReleaseFingerprint", "", "generatedReleaseFingerprint", releaseFingerprint, "comparisonResult", "changed")
	}
	complete(StageCompare, true, "release inputs changed")
	result.Changed = true

	logger.InfoContext(ctx, "pipeline stage started", "stage", StageGenerateDomains, "game_version", cfg.GameVersion, "master_sha256", masterHash)
	generated, err := GenerateDomains(ctx, GenerateOptions{Master: masterResult.Master, OutputRoot: candidateRoot, Domains: cfg.Domains})
	if err != nil {
		return result, fail(StageGenerateDomains, err)
	}
	complete(StageGenerateDomains, true, "")

	logger.InfoContext(ctx, "pipeline stage started", "stage", StageValidate, "game_version", cfg.GameVersion, "master_sha256", masterHash)
	if err := validation.Master(masterRoot); err != nil {
		return result, fail(StageValidate, fmt.Errorf("validate master dataset: %w", err))
	}
	if err := validation.GeneratedFiles(candidateRoot, generated.Files); err != nil {
		return result, fail(StageValidate, fmt.Errorf("validate domain datasets: %w", err))
	}
	sourceHash, err := sourceFingerprint(releaseFingerprint, cfg)
	if err != nil {
		return result, fail(StageValidate, err)
	}
	candidate, err := manifest.BuildCandidate(cfg.GameVersion, sourceHash, masterHash, candidateRoot, manualPayloads)
	if err != nil {
		return result, fail(StageValidate, err)
	}
	result.Candidate = candidate
	if err := manifest.WriteCandidate(filepath.Join(candidateRoot, "candidate.json"), candidate); err != nil {
		return result, fail(StageValidate, err)
	}
	report := validation.CandidateReport(candidateRoot, candidate, validation.Options{WarningsAsErrors: cfg.WarningsAsErrors})
	result.Warnings = append(result.Warnings, generated.Warnings...)
	result.Warnings = append(result.Warnings, report.Warnings()...)
	if err := report.Err(cfg.WarningsAsErrors); err != nil {
		return result, fail(StageValidate, err)
	}
	complete(StageValidate, false, "")

	logger.InfoContext(ctx, "pipeline stage started", "stage", StageAssembleRelease, "game_version", cfg.GameVersion, "master_sha256", masterHash)
	if dependencies.Clock == nil {
		return result, fail(StageAssembleRelease, fmt.Errorf("release clock is required"))
	}
	dataVersion := dependencies.Clock().UTC().Unix()
	releaseRoot := filepath.Join(workspace, "release")
	release, err := manifest.Assemble(candidateRoot, releaseRoot, candidate, dataVersion)
	if err != nil {
		return result, fail(StageAssembleRelease, err)
	}
	if err := validation.Release(releaseRoot, release); err != nil {
		return result, fail(StageAssembleRelease, fmt.Errorf("validate assembled release: %w", err))
	}
	result.Release = &release.ReleaseIdentity
	complete(StageAssembleRelease, true, "")

	if options.DryRun || !options.Publish {
		reason := "publication is not enabled"
		if options.DryRun {
			reason = "dry run"
		}
		skip(StagePublish, reason)
		skip(StageActivateRelease, reason)
		result.Outcome = contracts.PipelineOutcomeDryRun
		return result, nil
	}

	publisher, err := publish.New(dependencies.Store, dependencies.Clock)
	if err != nil {
		return result, fail(StagePublish, err)
	}
	logger.InfoContext(ctx, "pipeline stage started", "stage", StagePublish, "game_version", cfg.GameVersion, "master_sha256", masterHash, "data_version", release.DataVersion)
	if err := publisher.Upload(ctx, releaseRoot, release); err != nil {
		return result, fail(StagePublish, err)
	}
	complete(StagePublish, true, "")

	logger.InfoContext(ctx, "pipeline stage started", "stage", StageActivateRelease, "game_version", cfg.GameVersion, "master_sha256", masterHash, "data_version", release.DataVersion)
	if err := publisher.Activate(ctx, release); err != nil {
		return result, fail(StageActivateRelease, err)
	}
	complete(StageActivateRelease, true, "")
	result.Published = true
	result.Outcome = contracts.PipelineOutcomePublished
	return result, nil
}

func masterGenerationFailureStage(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "parse category"), strings.Contains(message, "duplicate canonical"), strings.Contains(message, "normalize"):
		return StageNormalize
	case strings.Contains(message, "write "), strings.Contains(message, "output"), strings.Contains(message, "promote"), strings.Contains(message, "working directory"):
		return StageWriteMaster
	default:
		return StageFetch
	}
}
