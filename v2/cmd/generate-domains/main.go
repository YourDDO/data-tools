package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"yourddo-data-tools/v2/internal/config"
	"yourddo-data-tools/v2/internal/dataset"
	pipelinepkg "yourddo-data-tools/v2/internal/pipeline"
	"yourddo-data-tools/v2/internal/validation"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("generate-domains: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	defaults, err := config.ReadEnvironment()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("generate-domains", flag.ContinueOnError)
	masterRoot := flags.String("master", defaults.MasterDir(), "master dataset directory")
	outputRoot := flags.String("output", defaults.DomainDir(), "domain output directory")
	domainsValue := flags.String("domains", strings.Join(defaults.Domains, ","), "comma-separated domains or all")
	if err := flags.Parse(args); err != nil {
		return err
	}
	domains := config.SplitList(*domainsValue)
	master, err := dataset.LoadMaster(*masterRoot)
	if err != nil {
		return err
	}
	result, err := pipelinepkg.GenerateDomains(context.Background(), pipelinepkg.GenerateOptions{
		Master: master, OutputRoot: *outputRoot, Domains: domains,
	})
	if err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		log.Printf("warning: %s", warning)
	}
	if err := validation.GeneratedFiles(*outputRoot, result.Files); err != nil {
		return err
	}
	log.Printf("generated %d domains in %s", len(result.Domains), *outputRoot)
	return nil
}
