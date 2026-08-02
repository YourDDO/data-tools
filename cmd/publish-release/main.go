package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"yourddo-data-tools/internal/config"
	"yourddo-data-tools/internal/publish"
	"yourddo-data-tools/internal/validation"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Printf("publish-release: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	defaults, err := config.ReadEnvironment()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("publish-release", flag.ContinueOnError)
	backend := flags.String("backend", "local", "publication backend: local or s3")
	environment := flags.String("environment", "development", "runtime environment")
	releaseRoot := flags.String("release-root", defaults.CandidateDir(), "candidate release directory")
	destination := flags.String("destination", "", "local publication root")
	region := flags.String("region", defaults.AWSRegion, "AWS region for S3 publication")
	bucket := flags.String("bucket", defaults.DataBucket, "S3 bucket for publication")
	if err := flags.Parse(args); err != nil {
		return err
	}
	selectedBackend := strings.ToLower(strings.TrimSpace(*backend))
	production := strings.EqualFold(strings.TrimSpace(*environment), "production")
	if production && selectedBackend != "s3" {
		return fmt.Errorf("production publishing requires the s3 backend")
	}
	if selectedBackend != "local" && selectedBackend != "s3" {
		return fmt.Errorf("unknown publication backend %q", *backend)
	}
	s3Config := defaults
	if selectedBackend == "s3" {
		s3Config.AWSRegion = strings.TrimSpace(*region)
		s3Config.DataBucket = strings.TrimSpace(*bucket)
		if err := s3Config.ValidateS3Publication(); err != nil {
			return err
		}
	}
	value, err := validation.DecodeCandidate(filepath.Join(*releaseRoot, "candidate.json"))
	if err != nil {
		return err
	}
	report := validation.CandidateReport(*releaseRoot, value, validation.Options{WarningsAsErrors: defaults.WarningsAsErrors})
	for _, warning := range report.Warnings() {
		log.Printf("warning: %s", warning)
	}
	if err := report.Err(defaults.WarningsAsErrors); err != nil {
		return err
	}
	var store publish.ObjectStore
	switch selectedBackend {
	case "local":
		store, err = publish.NewLocalStore(*destination)
	case "s3":
		awsConfig, loadErr := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(s3Config.AWSRegion))
		if loadErr != nil {
			return fmt.Errorf("load AWS configuration: %w", loadErr)
		}
		store, err = publish.NewS3Store(s3.NewFromConfig(awsConfig), s3Config.DataBucket)
	}
	if err != nil {
		return err
	}
	publisher, err := publish.New(store, time.Now)
	if err != nil {
		return err
	}
	release, err := publisher.Publish(ctx, *releaseRoot, value)
	if err != nil {
		return err
	}
	log.Printf("published immutable release %s/%d using %s backend", release.GameVersion, release.DataVersion, selectedBackend)
	return nil
}
