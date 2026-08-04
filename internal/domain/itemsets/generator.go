// Package itemsets generates shared item-set membership data.
package itemsets

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
)

const Name = "item-sets"

type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

func (Generator) Generate(_ context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	metadata, err := domain.WriteJSON(outputRoot, Name, "setBonusIndex.json", GenerateSetBonusIndexFromMaster(master))
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s set bonus index: %w", Name, err)
	}
	return domain.Result{Domain: Name, Files: []contracts.GeneratedFileMetadata{metadata}}, nil
}

type SetItem struct {
	Name     string `json:"name"`
	MinLevel int    `json:"minLevel"`
}

type SetBonusIndex map[string][]SetItem

type setItemCandidate struct {
	name      string
	pageTitle string
	minLevel  int
}

func GenerateSetBonusIndex(items []dataset.ItemData) SetBonusIndex {
	records := make([]dataset.ItemRecord, len(items))
	for index, item := range items {
		records[index].Item = item
	}
	return GenerateSetBonusIndexFromRecords(records)
}

func GenerateSetBonusIndexFromMaster(master dataset.Master) SetBonusIndex {
	return generateSetBonusIndex(master.Items, master.Augments)
}

// GenerateSetBonusIndexFromRecords preserves the original item-only helper for
// callers that do not have a complete canonical master dataset.
func GenerateSetBonusIndexFromRecords(records []dataset.ItemRecord) SetBonusIndex {
	return generateSetBonusIndex(records, nil)
}

func generateSetBonusIndex(records []dataset.ItemRecord, augments []dataset.AugmentRecord) SetBonusIndex {
	candidates := make(map[string][]setItemCandidate)
	seen := make(map[string]struct{})
	add := func(setName, identity string, candidate setItemCandidate) {
		setName = strings.TrimSpace(setName)
		if setName == "" || isTemplateControl(setName) || candidate.name == "" {
			return
		}
		key := setName + "\x00" + identity
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		candidates[setName] = append(candidates[setName], candidate)
	}

	for _, record := range records {
		item := record.Item
		itemName := strings.TrimSpace(item.Name)
		pageTitle := strings.TrimSpace(item.PageTitle)
		if strings.EqualFold(strings.TrimSpace(record.Category), "Filigrees") && pageTitle != "" {
			itemName = pageTitle
		}
		level := ParseMinimumLevel(item.MinLevel)
		identity := pageTitle
		if identity == "" {
			identity = itemName + "\x00" + strconv.Itoa(level)
		}
		for _, bonus := range item.SetBonus {
			add(bonus.Name, "item\x00"+identity, setItemCandidate{name: itemName, pageTitle: pageTitle, minLevel: level})
		}
	}

	for _, record := range augments {
		augment := record.Augment
		name := strings.TrimSpace(augment.Name)
		level := 0
		if augment.MinLevel != nil {
			level = *augment.MinLevel
		}
		identityData, _ := json.Marshal(augment)
		identity := string(identityData)
		for _, bonus := range augment.SetBonus {
			add(bonus.Name, "augment\x00"+identity, setItemCandidate{name: name, minLevel: level})
		}
	}

	result := make(SetBonusIndex, len(candidates))
	for setName, setCandidates := range candidates {
		nameCounts := make(map[string]int, len(setCandidates))
		for _, candidate := range setCandidates {
			nameCounts[candidate.name]++
		}
		items := make(map[string]SetItem, len(setCandidates))
		for _, candidate := range setCandidates {
			name := candidate.name
			if nameCounts[name] > 1 && candidate.pageTitle != "" {
				name = candidate.pageTitle
			}
			item := SetItem{Name: name, MinLevel: candidate.minLevel}
			if existing, exists := items[name]; !exists || item.MinLevel < existing.MinLevel {
				items[name] = item
			}
		}
		result[setName] = make([]SetItem, 0, len(items))
		for _, item := range items {
			result[setName] = append(result[setName], item)
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

func isTemplateControl(value string) bool {
	return strings.EqualFold(value, "true") || strings.EqualFold(value, "false")
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

// FiligreeSetNames returns the canonical set names from indexed master files.
func FiligreeSetNames(master dataset.Master) ([]string, error) {
	names := make(map[string]struct{})
	for _, file := range master.Files {
		if file.Kind != "filigree-sets" {
			continue
		}
		var sets []dataset.FiligreeSet
		if err := json.Unmarshal(file.Data, &sets); err != nil {
			return nil, fmt.Errorf("decode filigree sets %s: %w", file.Path, err)
		}
		for _, set := range sets {
			if name := strings.TrimSpace(set.Name); name != "" {
				names[name] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}
