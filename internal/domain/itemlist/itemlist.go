// Package itemlist contains the shared mechanics for domains that select
// canonical items by a domain-owned predicate.
package itemlist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
)

// Item retains both the typed fields used for selection and sorting and the
// exact canonical JSON value loaded from the master file.
type Item struct {
	dataset.ItemData
	raw json.RawMessage
}

// MarshalJSON copies the original master entry without projecting it through
// ItemData. Records constructed in memory fall back to the typed contract.
func (item Item) MarshalJSON() ([]byte, error) {
	if len(item.raw) != 0 {
		return item.raw, nil
	}
	return json.Marshal(item.ItemData)
}

func (item *Item) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &item.ItemData); err != nil {
		return err
	}
	item.raw = append(item.raw[:0], data...)
	return nil
}

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
	file, err := Write(outputRoot, g.name, "items.json", items)
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s: %w", g.name, err)
	}
	return domain.Result{Domain: g.name, Files: []contracts.GeneratedFileMetadata{file}}, nil
}

// Write emits an array while copying every selected item's original JSON value
// byte-for-byte from the master. Only array separators and the trailing newline
// are newly generated.
func Write(outputRoot, domainName, relative string, items []Item) (contracts.GeneratedFileMetadata, error) {
	var data bytes.Buffer
	data.WriteByte('[')
	for index, item := range items {
		encoded, err := item.MarshalJSON()
		if err != nil {
			return contracts.GeneratedFileMetadata{}, fmt.Errorf("encode item %q: %w", item.PageTitle, err)
		}
		if !json.Valid(encoded) {
			return contracts.GeneratedFileMetadata{}, fmt.Errorf("encode item %q: canonical JSON is invalid", item.PageTitle)
		}
		if index != 0 {
			data.WriteByte(',')
		}
		data.Write(encoded)
	}
	data.WriteString("]\n")
	return domain.WriteRawJSON(outputRoot, domainName, relative, data.Bytes())
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
	return Item{ItemData: record.Item, raw: append(json.RawMessage(nil), record.Raw...)}, nil
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
