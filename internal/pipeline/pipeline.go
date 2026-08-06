package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yourddo-data-tools/internal/compendium"
	"yourddo-data-tools/internal/config"
	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/hashing"
	"yourddo-data-tools/internal/manifest"
	"yourddo-data-tools/internal/manual"
	"yourddo-data-tools/internal/validation"
)

type Dependencies struct {
	Source compendium.Source
}

type Result struct {
	contracts.PipelineResult
	Root      string             `json:"-"`
	Candidate manifest.Candidate `json:"-"`
}

func Run(ctx context.Context, cfg config.Config, dependencies Dependencies) (result Result, returnErr error) {
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	if dependencies.Source == nil {
		return Result{}, fmt.Errorf("pipeline source is required")
	}
	stages := make([]contracts.PipelineStageResult, 0, 5)
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create pipeline work directory: %w", err)
	}
	staging, err := os.MkdirTemp(cfg.OutputDir, ".run-*")
	if err != nil {
		return Result{}, fmt.Errorf("create pipeline staging directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(staging); removeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("remove pipeline staging directory: %w", removeErr)
		}
	}()
	masterRoot := filepath.Join(staging, "master")
	generator, err := compendium.NewGenerator(dependencies.Source)
	if err != nil {
		return Result{}, err
	}
	master, err := generator.Generate(ctx, cfg.Categories, masterRoot)
	if err != nil {
		return Result{}, err
	}
	stages = append(stages, contracts.PipelineStageResult{Name: "fetch-master", Status: "succeeded", Changed: true})
	if err := validation.Master(masterRoot); err != nil {
		return Result{}, fmt.Errorf("validate master dataset: %w", err)
	}
	stages = append(stages, contracts.PipelineStageResult{Name: "validate-master", Status: "succeeded"})
	masterHash := master.SHA256
	manualPayloads, err := manual.Prepare(cfg.ManualInputDir, filepath.Join(staging, "manual"))
	if err != nil {
		return Result{}, fmt.Errorf("prepare manual payloads: %w", err)
	}
	sourceHash, err := sourceFingerprint(masterHash, manualPayloads, cfg)
	if err != nil {
		return Result{}, err
	}
	candidateRoot := cfg.CandidateDir()
	previous, previousErr := validation.DecodeCandidate(filepath.Join(candidateRoot, "candidate.json"))
	generated, err := GenerateDomains(ctx, GenerateOptions{Master: master.Master, OutputRoot: staging, Domains: cfg.Domains})
	if err != nil {
		return Result{}, err
	}
	stages = append(stages, contracts.PipelineStageResult{Name: "generate-domains", Status: "succeeded", Changed: true})
	if err := validation.GeneratedFiles(staging, generated.Files); err != nil {
		return Result{}, fmt.Errorf("validate domain datasets: %w", err)
	}
	stages = append(stages, contracts.PipelineStageResult{Name: "validate-domains", Status: "succeeded"})
	candidate, err := manifest.BuildCandidate(cfg.GameVersion, sourceHash, masterHash, staging, manualPayloads)
	if err != nil {
		return Result{}, err
	}
	if err := manifest.WriteCandidate(filepath.Join(staging, "candidate.json"), candidate); err != nil {
		return Result{}, err
	}
	validationOptions := validation.Options{WarningsAsErrors: cfg.WarningsAsErrors}
	if previousErr == nil {
		validationOptions.BaselineRoot = candidateRoot
		validationOptions.Baseline = &previous
	}
	report := validation.CandidateReport(staging, candidate, validationOptions)
	if err := report.Err(cfg.WarningsAsErrors); err != nil {
		return Result{}, fmt.Errorf("validate candidate release: %w", err)
	}
	stages = append(stages, contracts.PipelineStageResult{Name: "validate-candidate", Status: "succeeded"})
	if previousErr == nil && previous.ReleaseFingerprint == candidate.ReleaseFingerprint && previous.GameVersion == candidate.GameVersion {
		stages = append(stages, contracts.PipelineStageResult{Name: "detect-changes", Status: "succeeded", Message: "publishable artifacts and game version are unchanged"})
		return Result{
			PipelineResult: contracts.PipelineResult{Outcome: contracts.PipelineOutcomeNoChange, Changed: false, OutputDir: candidateRoot, Stages: stages},
			Root:           candidateRoot, Candidate: previous,
		}, nil
	}
	stages = append(stages, contracts.PipelineStageResult{Name: "detect-changes", Status: "succeeded", Changed: true})
	generated.Warnings = append(generated.Warnings, report.Warnings()...)
	sort.Strings(generated.Warnings)
	if err := os.RemoveAll(candidateRoot); err != nil {
		return Result{}, fmt.Errorf("replace previous candidate: %w", err)
	}
	if err := os.Rename(staging, candidateRoot); err != nil {
		return Result{}, fmt.Errorf("promote candidate: %w", err)
	}
	staging = ""
	return Result{
		PipelineResult: contracts.PipelineResult{
			Outcome: contracts.PipelineOutcomeDryRun, Changed: true, OutputDir: candidateRoot, Stages: stages, Warnings: generated.Warnings,
		},
		Root: candidateRoot, Candidate: candidate,
	}, nil
}

func sourceFingerprint(masterHash string, payloads []contracts.ManualPayloadMetadata, cfg config.Config) (string, error) {
	domains := append([]string(nil), cfg.Domains...)
	for index := range domains {
		domains[index] = strings.ToLower(strings.TrimSpace(domains[index]))
	}
	sort.Strings(domains)
	parts := []string{"pipeline-schema:5", "game-version:" + cfg.GameVersion, "master:" + masterHash, "domains:" + strings.Join(domains, ",")}
	orderedPayloads := append([]contracts.ManualPayloadMetadata(nil), payloads...)
	sort.Slice(orderedPayloads, func(i, j int) bool { return orderedPayloads[i].Path < orderedPayloads[j].Path })
	for _, payload := range orderedPayloads {
		parts = append(parts, "manual:"+payload.Name+":"+payload.Path+":"+payload.SHA256)
	}
	return hashing.Combine(parts...), nil
}
