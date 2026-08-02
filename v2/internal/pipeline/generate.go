package pipeline

import (
	"context"
	"fmt"
	"sort"

	"yourddo-data-tools/v2/internal/contracts"
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/registry"
)

type GenerateOptions struct {
	Master     dataset.Master
	OutputRoot string
	Domains    []string
}

type GenerateResult struct {
	Domains  []string
	Files    []contracts.GeneratedFileMetadata
	Warnings []string
}

// GenerateDomains passes the exact canonical contract returned by the master
// generator to each selected, source-independent generator.
func GenerateDomains(ctx context.Context, options GenerateOptions) (GenerateResult, error) {
	result := GenerateResult{Files: make([]contracts.GeneratedFileMetadata, 0), Warnings: make([]string, 0)}
	generators, err := registry.ResolveAll(options.Domains)
	if err != nil {
		return GenerateResult{}, err
	}
	for _, generator := range generators {
		generated, err := generator.Generate(ctx, options.Master, options.OutputRoot)
		if err != nil {
			return GenerateResult{}, fmt.Errorf("generate domain %s: %w", generator.Name(), err)
		}
		if generated.Domain != generator.Name() {
			return GenerateResult{}, fmt.Errorf("generate domain %s: result identifies domain %q", generator.Name(), generated.Domain)
		}
		result.Domains = append(result.Domains, generator.Name())
		result.Files = append(result.Files, generated.Files...)
		result.Warnings = append(result.Warnings, generated.Warnings...)
	}
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	sort.Strings(result.Warnings)
	return result, nil
}
