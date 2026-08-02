package zhentarim

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"yourddo-data-tools/v2/internal/contracts"
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain"
)

const Name = "zhentarim-attuned"

type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

func (Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	if err := ctx.Err(); err != nil {
		return domain.Result{}, err
	}
	items := make([]dataset.ItemData, 0, len(master.Items))
	for _, record := range master.Items {
		items = append(items, record.Item)
	}
	entries, warnings := Generate(items)
	metadata, err := domain.WriteJSON(outputRoot, Name, "upgrades.json", entries)
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s output %s: %w", Name, filepath.Join(Name, "upgrades.json"), err)
	}
	result := domain.Result{Domain: Name, Files: []contracts.GeneratedFileMetadata{metadata}, Warnings: warnings}
	domain.SortResult(&result)
	return result, nil
}

type Entry struct {
	Name           string                `json:"name"`
	EffectsRemoved []dataset.Enchantment `json:"effectsRemoved"`
	EffectsAdded   []dataset.Enchantment `json:"effectsAdded"`
}

var preferredOrder = map[string]int{
	"Necklace of Mystic Eidolons":  1,
	"Libram of Silver Magic":       2,
	"Lantern Ring":                 3,
	"Purple Dragon Shield":         4,
	"Magestar":                     5,
	"Manual of Stealthy Pilfering": 6,
}

func Generate(items []dataset.ItemData) ([]Entry, []string) {
	byName := make(map[string][]dataset.ItemData)
	for _, item := range items {
		if strings.TrimSpace(item.Name) != "" {
			byName[item.Name] = append(byName[item.Name], item)
		}
	}
	result := make([]Entry, 0)
	warnings := make([]string, 0)
	for name, variants := range byName {
		var base, upgraded *dataset.ItemData
		for index := range variants {
			variant := &variants[index]
			if hasAttunement(variant.Enchantments) && (base == nil || len(variant.PageTitle) < len(base.PageTitle) ||
				(len(variant.PageTitle) == len(base.PageTitle) && variant.PageTitle < base.PageTitle)) {
				base = variant
			}
			if strings.Contains(variant.PageTitle, "(Upgraded)") && (upgraded == nil || variant.PageTitle < upgraded.PageTitle) {
				upgraded = variant
			}
		}
		if base == nil {
			continue
		}
		if upgraded == nil {
			warnings = append(warnings, fmt.Sprintf("upgraded version of %q not found", name))
			continue
		}
		result = append(result, Entry{Name: name, EffectsRemoved: clean(base.Enchantments), EffectsAdded: clean(upgraded.Enchantments)})
	}
	sort.Slice(result, func(i, j int) bool {
		left, leftPreferred := preferredOrder[result[i].Name]
		right, rightPreferred := preferredOrder[result[j].Name]
		if leftPreferred != rightPreferred {
			return leftPreferred
		}
		if leftPreferred && left != right {
			return left < right
		}
		return result[i].Name < result[j].Name
	})
	sort.Strings(warnings)
	return result, warnings
}

func hasAttunement(enchantments []dataset.Enchantment) bool {
	for _, enchantment := range enchantments {
		if enchantment.Name == "Zhentarim Attuned" {
			return true
		}
	}
	return false
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
