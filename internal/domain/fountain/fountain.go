package fountain

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
	"yourddo-data-tools/internal/domain/itemlist"
)

const Name = "fountain-of-necrotic-might"

type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

func (Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	seedByName := make(map[string]struct{})
	items := make([]dataset.ItemData, 0, len(master.Items))
	for _, record := range master.Items {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}
		items = append(items, record.Item)
		if itemlist.HasEnchantment(record.Item, "Upgradeable Item (Black Abbot)") && strings.TrimSpace(record.Item.Name) != "" {
			seedByName[record.Item.Name] = struct{}{}
		}
	}
	names := make([]string, 0, len(seedByName))
	for name := range seedByName {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return dataset.NaturalLess(names[i], names[j]) })
	seed := make([]Entry, len(names))
	for index, name := range names {
		seed[index].Name = name
	}
	entries, warnings := Generate(seed, items)
	metadata, err := domain.WriteJSON(outputRoot, Name, "upgrades.json", entries)
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s: %w", Name, err)
	}
	result := domain.Result{Domain: Name, Files: []contracts.GeneratedFileMetadata{metadata}, Warnings: warnings}
	domain.SortResult(&result)
	return result, nil
}

type Entry struct {
	Name           string                `json:"name"`
	EffectsRemoved []dataset.Enchantment `json:"effectsRemoved,omitempty"`
	EffectsAdded   []dataset.Enchantment `json:"effectsAdded,omitempty"`
}

func Generate(seed []Entry, items []dataset.ItemData) ([]Entry, []string) {
	byName := make(map[string][]dataset.ItemData)
	for _, item := range items {
		if strings.TrimSpace(item.Name) != "" {
			byName[item.Name] = append(byName[item.Name], item)
		}
	}
	result := append([]Entry(nil), seed...)
	warnings := make([]string, 0)
	for index := range result {
		variants := byName[result[index].Name]
		if len(variants) == 0 {
			warnings = append(warnings, fmt.Sprintf("item %q not found in master dataset", result[index].Name))
			continue
		}
		var base, upgraded *dataset.ItemData
		for variantIndex := range variants {
			variant := &variants[variantIndex]
			if strings.Contains(variant.PageTitle, "(Upgraded)") {
				if upgraded == nil || variant.PageTitle < upgraded.PageTitle {
					upgraded = variant
				}
			} else if base == nil || len(variant.PageTitle) < len(base.PageTitle) ||
				(len(variant.PageTitle) == len(base.PageTitle) && variant.PageTitle < base.PageTitle) {
				base = variant
			}
		}
		if base != nil {
			result[index].EffectsRemoved = clean(base.Enchantments)
		}
		if upgraded != nil {
			result[index].EffectsAdded = clean(upgraded.Enchantments)
		} else {
			warnings = append(warnings, fmt.Sprintf("upgraded version of %q not found", result[index].Name))
		}
	}
	sort.Strings(warnings)
	return result, warnings
}

func clean(enchantments []dataset.Enchantment) []dataset.Enchantment {
	result := make([]dataset.Enchantment, 0, len(enchantments))
	for _, enchantment := range enchantments {
		name := strings.ToLower(strings.TrimSpace(enchantment.Name))
		if name == "" || strings.Contains(name, "upgradeable item") || strings.HasPrefix(name, "upgrade:") {
			continue
		}
		result = append(result, enchantment)
	}
	return result
}
