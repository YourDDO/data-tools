package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"yourddo-data-tools/internal/compendium"
	"yourddo-data-tools/internal/config"
	"yourddo-data-tools/internal/validation"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Printf("fetch-master: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	defaults, err := config.ReadEnvironment()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("fetch-master", flag.ContinueOnError)
	baseURL := flags.String("base-url", defaults.CompendiumBaseURL, "Compendium base URL")
	apiPath := flags.String("api-path", defaults.CompendiumAPIPath, "Compendium API path")
	output := flags.String("output", defaults.MasterDir(), "master dataset output directory")
	categories := flags.String("categories", strings.Join(defaults.Categories, ","), "comma-separated Compendium categories")
	if err := flags.Parse(args); err != nil {
		return err
	}
	defaults.CompendiumBaseURL = *baseURL
	defaults.CompendiumAPIPath = *apiPath
	defaults.Categories = config.SplitList(*categories)
	if err := defaults.ValidateFetch(); err != nil {
		return err
	}
	client, err := compendium.NewClient(defaults.CompendiumAPIURL())
	if err != nil {
		return err
	}
	generator, err := compendium.NewGenerator(client)
	if err != nil {
		return err
	}
	result, err := generator.GenerateReplacing(ctx, defaults.Categories, *output)
	if err != nil {
		return err
	}
	if err := validation.Master(*output); err != nil {
		return err
	}
	log.Printf("wrote %d master dataset files to %s (sha256 %s)", len(result.Master.Index.Files), *output, result.SHA256)
	return nil
}
