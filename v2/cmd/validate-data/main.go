package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"yourddo-data-tools/v2/internal/config"
	"yourddo-data-tools/v2/internal/validation"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("validate-data: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	defaults, err := config.ReadEnvironment()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("validate-data", flag.ContinueOnError)
	masterRoot := flags.String("master", defaults.MasterDir(), "master dataset directory")
	domainRoot := flags.String("domains-root", defaults.DomainDir(), "domain dataset directory")
	domainsValue := flags.String("domains", strings.Join(defaults.Domains, ","), "comma-separated domains or all")
	manifestPath := flags.String("manifest", "", "optional candidate manifest")
	releaseRoot := flags.String("release-root", "", "root corresponding to --manifest")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := validation.Master(*masterRoot); err != nil {
		return err
	}
	if err := validation.Domains(*domainRoot, config.SplitList(*domainsValue)); err != nil {
		return err
	}
	if *manifestPath != "" {
		value, err := validation.DecodeManifest(*manifestPath)
		if err != nil {
			return err
		}
		root := *releaseRoot
		if root == "" {
			root = defaults.OutputDir
		}
		if err := validation.Release(root, value); err != nil {
			return err
		}
	}
	log.Printf("all selected datasets are valid")
	return nil
}
