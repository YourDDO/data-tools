package pipeline

import (
	"fmt"
	"path/filepath"
	"sort"

	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain/itemsets"
	"yourddo-data-tools/internal/manual"
)

type itemSetCoverageWarning struct {
	source          string
	manualFound     bool
	missingSetNames []string
}

func itemSetDefinitionCoverage(master dataset.Master, manualRoot string) ([]itemSetCoverageWarning, error) {
	manualNames, found, err := manual.LoadItemSetNames(filepath.Join(manualRoot, manual.ItemSetEnchantmentsFile))
	if err != nil {
		return nil, fmt.Errorf("validate manual item-set definitions: %w", err)
	}
	defined := make(map[string]struct{}, len(manualNames))
	for _, name := range manualNames {
		defined[name] = struct{}{}
	}

	index := itemsets.GenerateSetBonusIndexFromMaster(master)
	indexNames := make([]string, 0, len(index))
	for name := range index {
		indexNames = append(indexNames, name)
	}
	sort.Strings(indexNames)
	filigreeNames, err := itemsets.FiligreeSetNames(master)
	if err != nil {
		return nil, err
	}

	warnings := make([]itemSetCoverageWarning, 0, 2)
	for _, source := range []struct {
		name string
		sets []string
	}{
		{name: "setBonusIndex", sets: indexNames},
		{name: "filigreeSets", sets: filigreeNames},
	} {
		missing := make([]string, 0)
		for _, name := range source.sets {
			if _, exists := defined[name]; !exists {
				missing = append(missing, name)
			}
		}
		if len(missing) != 0 {
			warnings = append(warnings, itemSetCoverageWarning{
				source: source.name, manualFound: found, missingSetNames: missing,
			})
		}
	}
	return warnings, nil
}
