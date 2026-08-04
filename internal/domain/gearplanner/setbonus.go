package gearplanner

import (
	"context"
	"fmt"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
	"yourddo-data-tools/internal/domain/itemsets"
)

const Name = "gear-planner"

// Generator republishes every indexed canonical record file without dropping
// fields and adds the derived set-bonus lookup used by the planner.
type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

func (Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	result := domain.Result{Domain: Name, Files: make([]contracts.GeneratedFileMetadata, 0, len(master.Files)+2)}
	for _, file := range master.Files {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}
		metadata, err := domain.WriteCanonical(outputRoot, Name, file.Path, file.Data)
		if err != nil {
			return domain.Result{}, fmt.Errorf("domain %s canonical file %s: %w", Name, file.Path, err)
		}
		result.Files = append(result.Files, metadata)
	}
	var metadata contracts.GeneratedFileMetadata
	var err error
	if len(master.IndexData) != 0 {
		metadata, err = domain.WriteCanonical(outputRoot, Name, dataset.MasterIndexName, master.IndexData)
	} else {
		metadata, err = domain.WriteJSON(outputRoot, Name, dataset.MasterIndexName, master.Index)
	}
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s canonical master index: %w", Name, err)
	}
	result.Files = append(result.Files, metadata)
	metadata, err = domain.WriteJSON(outputRoot, Name, "setBonusIndex.json", itemsets.GenerateSetBonusIndexFromMaster(master))
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s set bonus index: %w", Name, err)
	}
	result.Files = append(result.Files, metadata)
	domain.SortResult(&result)
	return result, nil
}

type SetItem = itemsets.SetItem
type SetBonusIndex = itemsets.SetBonusIndex

func GenerateSetBonusIndex(items []dataset.ItemData) SetBonusIndex {
	return itemsets.GenerateSetBonusIndex(items)
}

// GenerateSetBonusIndexFromRecords uses the canonical page title for
// filigrees. Compendium filigree templates contain a number of incorrect or
// duplicated name fields; using those names would collapse distinct pages and
// silently remove entries from the index. Other item types keep their display
// name unless multiple canonical records in the same set share it, in which
// case their page titles disambiguate the variants without dropping data.
func GenerateSetBonusIndexFromRecords(records []dataset.ItemRecord) SetBonusIndex {
	return itemsets.GenerateSetBonusIndexFromRecords(records)
}

func ParseMinimumLevel(value string) int {
	return itemsets.ParseMinimumLevel(value)
}
