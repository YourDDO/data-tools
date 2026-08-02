// Package itemlist contains the shared mechanics for domains that select
// canonical items by a domain-owned predicate.
package itemlist

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"yourddo-data-tools/v2/internal/contracts"
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain"
)

// Item is the compact contract used by crafting-selection datasets.
//
// PageTitle is the stable canonical record identity; Name is the player-facing
// label; Type and MinLevel let clients place and level-gate the item; and
// Enchantments contain both the crafting marker and the effects a crafting UI
// presents. Augments is included only by Dinosaur Bone, whose recipes operate
// on those slots. Source, history, prices, images, and durability are omitted
// because none of these domain rules or consumers use them.
type Item struct {
	PageTitle    string                `json:"pageTitle"`
	Name         string                `json:"name"`
	Type         string                `json:"type"`
	MinLevel     string                `json:"minLevel"`
	Enchantments []dataset.Enchantment `json:"enchantments"`
	Augments     []dataset.AugmentItem `json:"augments,omitempty"`
}

type Matcher func(dataset.ItemData) bool

type Generator struct {
	name            string
	match           Matcher
	includeAugments bool
}

func New(name string, match Matcher) Generator {
	return Generator{name: name, match: match}
}

func NewWithAugments(name string, match Matcher) Generator {
	return Generator{name: name, match: match, includeAugments: true}
}

func (g Generator) Name() string { return g.name }

func (g Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	selected := Filter(master.Items, g.match)
	items := make([]Item, 0, len(selected))
	for _, record := range selected {
		if err := ctx.Err(); err != nil {
			return domain.Result{}, err
		}
		item, err := Transform(record, g.includeAugments)
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

// Transform projects a selected canonical record into the compact domain
// contract. Canonical normalization is deliberately not repeated here.
func Transform(record dataset.ItemRecord, includeAugments bool) (Item, error) {
	if strings.TrimSpace(record.Item.PageTitle) == "" || strings.TrimSpace(record.Item.Name) == "" {
		return Item{}, fmt.Errorf("pageTitle and name are required")
	}
	item := Item{
		PageTitle: record.Item.PageTitle, Name: record.Item.Name, Type: record.Item.Type,
		MinLevel: record.Item.MinLevel, Enchantments: append([]dataset.Enchantment(nil), record.Item.Enchantments...),
	}
	if includeAugments {
		item.Augments = append([]dataset.AugmentItem(nil), record.Item.Augments...)
	}
	return item, nil
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
