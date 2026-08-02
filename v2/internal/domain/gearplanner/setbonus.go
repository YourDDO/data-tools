package gearplanner

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"yourddo-data-tools/v2/internal/contracts"
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain"
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
	metadata, err = domain.WriteJSON(outputRoot, Name, "setBonusIndex.json", GenerateSetBonusIndexFromRecords(master.Items))
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s set bonus index: %w", Name, err)
	}
	result.Files = append(result.Files, metadata)
	domain.SortResult(&result)
	return result, nil
}

type SetItem struct {
	Name     string `json:"name"`
	MinLevel int    `json:"minLevel"`
}

type SetBonusIndex map[string][]SetItem

func GenerateSetBonusIndex(items []dataset.ItemData) SetBonusIndex {
	records := make([]dataset.ItemRecord, len(items))
	for index, item := range items {
		records[index].Item = item
	}
	return GenerateSetBonusIndexFromRecords(records)
}

// GenerateSetBonusIndexFromRecords uses the canonical page title for
// filigrees. Compendium filigree templates contain a number of incorrect or
// duplicated name fields; using those names would collapse distinct pages and
// silently remove entries from the index.
func GenerateSetBonusIndexFromRecords(records []dataset.ItemRecord) SetBonusIndex {
	result := make(SetBonusIndex)
	seen := make(map[string]struct{})
	for _, record := range records {
		item := record.Item
		itemName := strings.TrimSpace(item.Name)
		identity := itemName
		if pageTitle := strings.TrimSpace(item.PageTitle); strings.EqualFold(strings.TrimSpace(record.Category), "Filigrees") && pageTitle != "" {
			itemName = pageTitle
			identity = pageTitle
		}
		for _, bonus := range item.SetBonus {
			name := strings.TrimSpace(bonus.Name)
			if name == "" || itemName == "" {
				continue
			}
			level := ParseMinimumLevel(item.MinLevel)
			key := name + "\x00" + identity + "\x00" + strconv.Itoa(level)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result[name] = append(result[name], SetItem{Name: itemName, MinLevel: level})
		}
	}
	for name := range result {
		sort.Slice(result[name], func(i, j int) bool {
			if result[name][i].MinLevel != result[name][j].MinLevel {
				return result[name][i].MinLevel < result[name][j].MinLevel
			}
			return result[name][i].Name < result[name][j].Name
		})
	}
	return result
}

func ParseMinimumLevel(value string) int {
	var digits strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		} else if digits.Len() > 0 {
			break
		}
	}
	level, _ := strconv.Atoi(digits.String())
	return level
}
