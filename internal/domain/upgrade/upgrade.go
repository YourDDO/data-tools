// Package upgrade contains shared pairing and effect-delta transformation for
// crafting domains whose canonical records have base and upgraded variants.
package upgrade

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
)

// Entry contains only fields used when applying an upgrade: the display name
// identifies the choice and the two effect lists describe what is replaced.
type Entry struct {
	Name           string                `json:"name"`
	EffectsRemoved []dataset.Enchantment `json:"effectsRemoved"`
	EffectsAdded   []dataset.Enchantment `json:"effectsAdded"`
}

type Marker func(dataset.ItemData) bool

func Generate(ctx context.Context, domainName string, marker Marker, master dataset.Master, outputRoot string) (domain.Result, error) {
	byName := make(map[string][]dataset.ItemRecord)
	for _, record := range master.Items {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}
		if strings.TrimSpace(record.Item.Name) != "" {
			byName[record.Item.Name] = append(byName[record.Item.Name], record)
		}
	}
	entries := make([]Entry, 0)
	warnings := make([]string, 0)
	for name, variants := range byName {
		var base, upgraded *dataset.ItemRecord
		for index := range variants {
			candidate := &variants[index]
			if marker(candidate.Item) && betterBase(candidate, base) {
				base = candidate
			}
			if strings.Contains(candidate.Item.PageTitle, "(Upgraded)") &&
				(upgraded == nil || candidate.Item.PageTitle < upgraded.Item.PageTitle) {
				upgraded = candidate
			}
		}
		if base == nil {
			continue
		}
		if strings.TrimSpace(base.Item.PageTitle) == "" {
			return domain.Result{}, fmt.Errorf("domain %s record %s: pageTitle is required", domainName, base.Source())
		}
		if upgraded == nil {
			warnings = append(warnings, fmt.Sprintf("domain %s record %s: upgraded version of %q not found", domainName, base.Source(), name))
			continue
		}
		entries = append(entries, Entry{Name: name, EffectsRemoved: clean(base.Item.Enchantments), EffectsAdded: clean(upgraded.Item.Enchantments)})
	}
	sort.Slice(entries, func(i, j int) bool { return dataset.NaturalLess(entries[i].Name, entries[j].Name) })
	file, err := domain.WriteJSON(outputRoot, domainName, "upgrades.json", entries)
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s: %w", domainName, err)
	}
	result := domain.Result{Domain: domainName, Files: []contracts.GeneratedFileMetadata{file}, Warnings: warnings}
	domain.SortResult(&result)
	return result, nil
}

func betterBase(candidate, current *dataset.ItemRecord) bool {
	return current == nil || len(candidate.Item.PageTitle) < len(current.Item.PageTitle) ||
		(len(candidate.Item.PageTitle) == len(current.Item.PageTitle) && candidate.Item.PageTitle < current.Item.PageTitle)
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
