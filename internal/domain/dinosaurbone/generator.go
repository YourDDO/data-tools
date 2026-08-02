// Package dinosaurbone owns Dinosaur Bone item and augment selection.
package dinosaurbone

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

const (
	Name               = "dinosaur-bone"
	dinosaurTypePrefix = "Isle of Dread:"
)

type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

// Ingredient contains only the recipe identity and quantity needed to craft
// an augment. Other AugmentItem fields do not apply to ingredients.
type Ingredient struct {
	Name           string `json:"name"`
	IngredientType string `json:"ingredientType,omitempty"`
	Quantity       *int   `json:"quantity,omitempty"`
}

// Augment is the Dinosaur Bone crafting contract. Name and AugmentType identify
// the choice and compatible slot; MinLevel gates it; effects and set bonuses
// describe the result; and CraftedIn, Requirements, and FoundIn explain how it
// is acquired. Binding, weight, update history, and unrelated master fields are
// intentionally omitted.
type Augment struct {
	Name           string                          `json:"name"`
	AugmentType    string                          `json:"augmentType"`
	MinLevel       *int                            `json:"minLevel,omitempty"`
	Description    string                          `json:"description,omitempty"`
	FoundIn        []string                        `json:"foundIn,omitempty"`
	EffectsAdded   []dataset.PartialEnhancementOut `json:"effectsAdded,omitempty"`
	EffectsRemoved []dataset.PartialEnhancementOut `json:"effectsRemoved,omitempty"`
	CraftedIn      string                          `json:"craftedIn,omitempty"`
	Requirements   []Ingredient                    `json:"requirements,omitempty"`
	SetBonus       []dataset.SetBonusOut           `json:"setBonus,omitempty"`
}

func (Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	items, err := selectItems(ctx, master.Items)
	if err != nil {
		return domain.Result{}, err
	}
	augments, err := selectAugments(ctx, master.Augments)
	if err != nil {
		return domain.Result{}, err
	}
	itemFile, err := domain.WriteJSON(outputRoot, Name, "items.json", items)
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s items: %w", Name, err)
	}
	augmentFile, err := domain.WriteJSON(outputRoot, Name, "augments.json", augments)
	if err != nil {
		return domain.Result{}, fmt.Errorf("domain %s augments: %w", Name, err)
	}
	result := domain.Result{Domain: Name, Files: []contracts.GeneratedFileMetadata{itemFile, augmentFile}}
	domain.SortResult(&result)
	return result, nil
}

// selectItems filters on the canonical slot contract, which is shared by
// weapons, armor, jewelry, and other accessories. Drop-source filtering would
// incorrectly exclude accessories that are found as loot.
func selectItems(ctx context.Context, records []dataset.ItemRecord) ([]itemlist.Item, error) {
	selected := itemlist.Filter(records, eligibleItem)
	items := make([]itemlist.Item, 0, len(selected))
	for _, record := range selected {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item, err := itemlist.Transform(record, true)
		if err != nil {
			return nil, fmt.Errorf("domain %s record %s: %w", Name, record.Source(), err)
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return dataset.NaturalLess(items[i].Name, items[j].Name)
		}
		return items[i].PageTitle < items[j].PageTitle
	})
	return items, nil
}

func eligibleItem(item dataset.ItemData) bool {
	for _, augment := range item.Augments {
		if strings.HasPrefix(augment.AugmentType, dinosaurTypePrefix) && strings.Contains(augment.AugmentType, " Slot") {
			return true
		}
	}
	return false
}

func selectAugments(ctx context.Context, records []dataset.AugmentRecord) ([]Augment, error) {
	augments := make([]Augment, 0)
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(record.Augment.AugmentType, dinosaurTypePrefix) {
			continue
		}
		if strings.TrimSpace(record.Augment.Name) == "" {
			return nil, fmt.Errorf("domain %s record %s: name is required", Name, record.Source())
		}
		augment := Augment{
			Name: record.Augment.Name, AugmentType: record.Augment.AugmentType,
			MinLevel: record.Augment.MinLevel, Description: record.Augment.Description,
			FoundIn:        append([]string(nil), record.Augment.FoundIn...),
			EffectsAdded:   append([]dataset.PartialEnhancementOut(nil), record.Augment.EffectsAdded...),
			EffectsRemoved: append([]dataset.PartialEnhancementOut(nil), record.Augment.EffectsRemoved...),
			CraftedIn:      record.Augment.CraftedIn, SetBonus: append([]dataset.SetBonusOut(nil), record.Augment.SetBonus...),
		}
		for _, requirement := range record.Augment.Requirements {
			augment.Requirements = append(augment.Requirements, Ingredient{
				Name: requirement.Name, IngredientType: requirement.IngredientType, Quantity: requirement.Quantity,
			})
		}
		augments = append(augments, augment)
	}
	sort.Slice(augments, func(i, j int) bool {
		if augments[i].Name != augments[j].Name {
			return dataset.NaturalLess(augments[i].Name, augments[j].Name)
		}
		return augments[i].AugmentType < augments[j].AugmentType
	})
	return augments, nil
}
