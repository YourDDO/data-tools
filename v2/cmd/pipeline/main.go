package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"yourddo-data-tools/v2/internal/compendium"
	"yourddo-data-tools/v2/internal/config"
	"yourddo-data-tools/v2/internal/contracts"
	pipelinepkg "yourddo-data-tools/v2/internal/pipeline"
	"yourddo-data-tools/v2/internal/publish"
)

type commandDependencies struct {
	stdout io.Writer
	stderr io.Writer
	clock  func() time.Time
}

func main() {
	dependencies := commandDependencies{stdout: os.Stdout, stderr: os.Stderr, clock: time.Now}
	if err := run(context.Background(), os.Args[1:], dependencies); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, dependencies commandDependencies) error {
	if dependencies.stdout == nil {
		dependencies.stdout = io.Discard
	}
	if dependencies.stderr == nil {
		dependencies.stderr = io.Discard
	}
	if dependencies.clock == nil {
		dependencies.clock = time.Now
	}
	logger := slog.New(slog.NewJSONHandler(dependencies.stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.ReadEnvironment()
	if err != nil {
		return configurationFailure(ctx, logger, dependencies.stdout, "", err)
	}
	flags := flag.NewFlagSet("pipeline", flag.ContinueOnError)
	flags.SetOutput(dependencies.stderr)
	flags.Usage = func() { _, _ = fmt.Fprintln(dependencies.stderr, "usage: pipeline [options]") }
	flags.StringVar(&cfg.CompendiumBaseURL, "base-url", cfg.CompendiumBaseURL, "Compendium base URL")
	flags.StringVar(&cfg.CompendiumAPIPath, "api-path", cfg.CompendiumAPIPath, "Compendium API path")
	flags.StringVar(&cfg.OutputDir, "output-dir", cfg.OutputDir, "parent directory for isolated pipeline work")
	flags.StringVar(&cfg.GameVersion, "game-version", cfg.GameVersion, "numeric game version")
	categories := flags.String("categories", strings.Join(cfg.Categories, ","), "comma-separated Compendium categories")
	domains := flags.String("domains", strings.Join(cfg.Domains, ","), "comma-separated domains or all")
	dryRun := flags.Bool("dry-run", false, "generate, validate, and assemble without publication writes")
	publishEnabled := flags.Bool("publish", cfg.PublishEnabled, "publish and activate the validated release")
	backend := flags.String("backend", "local", "publication backend: local or s3")
	region := flags.String("region", cfg.AWSRegion, "AWS region for S3 publication")
	bucket := flags.String("bucket", cfg.DataBucket, "S3 bucket for publication")
	var publishRoot string
	flags.StringVar(&publishRoot, "publish-root", "", "local filesystem publication root")
	flags.StringVar(&publishRoot, "destination", "", "alias for --publish-root")
	preserveWork := flags.Bool("debug-preserve", false, "preserve the isolated work directory for debugging")
	preserveWorkAlias := flags.Bool("preserve-work", false, "alias for --debug-preserve")
	debugAlias := flags.Bool("debug", false, "alias for --debug-preserve")
	if err := flags.Parse(args); err != nil {
		return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, err)
	}
	cfg.Categories = config.SplitList(*categories)
	cfg.Domains = config.SplitList(*domains)
	requestedPublish := *publishEnabled
	cfg.PublishEnabled = false // The selected command backend owns publication validation.

	selectedBackend := strings.ToLower(strings.TrimSpace(*backend))
	if selectedBackend != "local" && selectedBackend != "s3" {
		err := fmt.Errorf("publication backend %q is not available", *backend)
		return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, err)
	}
	var active pipelinepkg.ActiveHashReader
	var store publish.ObjectStore
	switch selectedBackend {
	case "local":
		if strings.TrimSpace(publishRoot) != "" {
			localStore, storeErr := publish.NewLocalStore(publishRoot)
			if storeErr != nil {
				return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, storeErr)
			}
			active = localStore
			store = localStore
		}
		if requestedPublish && store == nil {
			err := fmt.Errorf("--publish-root is required when local publication is enabled")
			return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, err)
		}
	case "s3":
		cfg.AWSRegion = strings.TrimSpace(*region)
		cfg.DataBucket = strings.TrimSpace(*bucket)
		if err := cfg.ValidateS3Publication(); err != nil {
			return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, err)
		}
		awsConfig, loadErr := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
		if loadErr != nil {
			return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, fmt.Errorf("load AWS configuration: %w", loadErr))
		}
		s3Store, storeErr := publish.NewS3Store(s3.NewFromConfig(awsConfig), cfg.DataBucket)
		if storeErr != nil {
			return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, storeErr)
		}
		active = s3Store
		store = s3Store
	}

	// Validate before constructing the client so malformed or credential-bearing
	// URLs can never appear in client errors or structured logs.
	if err := cfg.Validate(); err != nil {
		return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, err)
	}
	client, err := compendium.NewClient(cfg.CompendiumAPIURL())
	if err != nil {
		return configurationFailure(ctx, logger, dependencies.stdout, cfg.GameVersion, err)
	}
	result, pipelineErr := pipelinepkg.Execute(ctx, cfg, pipelinepkg.ExecuteOptions{
		DryRun: *dryRun, Publish: requestedPublish, PreserveWork: *preserveWork || *preserveWorkAlias || *debugAlias,
	}, pipelinepkg.OrchestratorDependencies{
		Source: client, Active: active, Store: store, Clock: dependencies.clock, Logger: logger,
	})
	if encodeErr := json.NewEncoder(dependencies.stdout).Encode(result.PipelineResult); encodeErr != nil {
		if pipelineErr != nil {
			return fmt.Errorf("%w; encode pipeline result: %v", pipelineErr, encodeErr)
		}
		return fmt.Errorf("encode pipeline result: %w", encodeErr)
	}
	return pipelineErr
}

func configurationFailure(ctx context.Context, logger *slog.Logger, output io.Writer, gameVersion string, failure error) error {
	stageFailure := &pipelinepkg.StageError{Stage: pipelinepkg.StageConfiguration, GameVersion: gameVersion, Err: failure}
	logger.ErrorContext(ctx, "pipeline configuration failed", "stage", pipelinepkg.StageConfiguration, "game_version", gameVersion, "error", stageFailure)
	result := contracts.PipelineResult{
		Outcome: contracts.PipelineOutcomeFailed,
		Stages:  []contracts.PipelineStageResult{{Name: pipelinepkg.StageConfiguration, Status: "failed", Message: "stage failed"}},
	}
	if err := json.NewEncoder(output).Encode(result); err != nil {
		return errors.Join(stageFailure, fmt.Errorf("encode pipeline result: %w", err))
	}
	return stageFailure
}
