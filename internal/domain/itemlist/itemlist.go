// Package itemlist contains the shared mechanics for domains that select
// canonical items by a domain-owned predicate.
package itemlist

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
)

// Item is the complete canonical item contract. Domain lists select canonical
// items but do not project them into a smaller, domain-specific shape.
type Item = dataset.ItemData

type Matcher func(dataset.ItemData) bool

type Generator struct {
	name  string
	match Matcher
}

func New(name string, match Matcher) Generator {
	return Generator{name: name, match: match}
}

func (g Generator) Name() string { return g.name }

func (g Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	selected := Filter(master.Items, g.match)
	items := make([]Item, 0, len(selected))
	for _, record := range selected {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}
		item, err := Transform(record)
		if err != nil {
			return domain.Result{}, fmt.Errorf("domain %s record %s: %w", g.name, record.Source(), err)
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return dataset.NaturalLess(items[i].Name, items[j].Name)
		}
		return items[i].PageTitle < items[j].PageTitle
	})
	file, err := domain.WriteJSON(outputRoot, g.name, "items.json", items)
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s: %w", g.name, err)
	}
	return domain.Result{Domain: g.name, Files: []contracts.GeneratedFileMetadata{file}}, nil
}

// Filter performs selection only; it does not alter canonical records.
func Filter(records []dataset.ItemRecord, match Matcher) []dataset.ItemRecord {
	selected := make([]dataset.ItemRecord, 0)
	for _, record := range records {
		if match(record.Item) {
			selected = append(selected, record)
		}
	}
	return selected
}

// Transform validates and copies a selected canonical record without dropping
// fields. Canonical normalization is deliberately not repeated here.
func Transform(record dataset.ItemRecord) (Item, error) {
	if strings.TrimSpace(record.Item.PageTitle) == "" || strings.TrimSpace(record.Item.Name) == "" {
		return Item{}, fmt.Errorf("pageTitle and name are required")
	}
	return record.Item, nil
}

func HasEnchantment(item dataset.ItemData, name string) bool {
	for _, enchantment := range item.Enchantments {
		if enchantment.Name == name {
			return true
		}
	}
	return false
}

func HasEnchantmentPrefix(item dataset.ItemData, prefix string) bool {
	for _, enchantment := range item.Enchantments {
		if strings.HasPrefix(enchantment.Name, prefix) {
			return true
		}
	}
	return false
}
