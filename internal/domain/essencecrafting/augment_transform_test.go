package essencecrafting

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestTransformAugmentEffectThreatPercentModifiers(t *testing.T) {
	for _, input := range []dataset.PartialEnhancementOut{
		{Name: "Melee Threat", Modifier: -0.2},
		{Name: "Ranged Threat", Modifier: "-0.2"},
		{Name: "Spell Threat", Modifier: "-20%"},
	} {
		effect, err := transformAugmentEffect("augment-test", input)
		if err != nil {
			t.Fatal(err)
		}
		if effect.Modifier == nil || effect.Modifier.Kind != "fixed" || effect.Modifier.Unit != "percent" || effect.Modifier.Value != -20.0 {
			t.Fatalf("transformAugmentEffect(%#v) = %#v, want fixed -20 percent", input, effect)
		}
	}
}

func TestTransformAugmentsRejectsDuplicateSemanticEffectsWithDetails(t *testing.T) {
	level := 30
	master := dataset.Master{Augments: []dataset.AugmentRecord{{
		File: "augment.json",
		Augment: dataset.AugmentItem{
			Name:        "Duplicate Effect",
			AugmentType: "Yellow",
			MinLevel:    &level,
			EffectsAdded: []dataset.PartialEnhancementOut{
				{Name: "Occultation", Modifier: -0.2},
				{Name: "Occultation", Modifier: -0.1},
			},
		},
	}}}

	_, _, err := transformAugments(context.Background(), master)
	if err == nil {
		t.Fatal("transformAugments() unexpectedly succeeded")
	}
	for _, want := range []string{
		"augment.json#Duplicate Effect",
		"repeats semantic effect \"Occultation\" with empty bonus",
		"effects[0] and effects[1]",
		"generated ID \"effect-",
		"first modifier=-0.2",
		"duplicate modifier=-0.1",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want substring %q", err, want)
		}
	}
}

func TestTransformAugmentsIncludesItemSetMembership(t *testing.T) {
	level := 1
	master := dataset.Master{Augments: []dataset.AugmentRecord{{
		File: "augment.json",
		Augment: dataset.AugmentItem{
			Name:        "Set Augment: Example Set Name",
			AugmentType: "Colorless",
			MinLevel:    &level,
			SetBonus:    []dataset.SetBonusOut{{Name: "Example Set Name"}},
		},
	}}}

	first, err := build(context.Background(), master, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := build(context.Background(), master, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("transformAugments() is not deterministic: %#v != %#v", first, second)
	}
	if len(first.Augments) != 1 || len(first.Augments[0].Effects) != 1 {
		t.Fatalf("generated augments = %#v, want one augment with one effect", first.Augments)
	}
	effect := first.Augments[0].Effects[0]
	if effect.DisplayName != "Item Set: Example Set Name" || effect.BonusTypeID != "" || effect.Modifier != nil {
		t.Fatalf("item set effect = %#v", effect)
	}
	wantID := effectID(first.Augments[0].ID, "", "Item Set: Example Set Name")
	if effect.ID != wantID {
		t.Fatalf("item set effect ID = %q, want %q", effect.ID, wantID)
	}
}

func TestTransformAugmentsDoesNotDuplicateItemSetCompatibilityMarker(t *testing.T) {
	level := 1
	master := dataset.Master{Augments: []dataset.AugmentRecord{{
		File: "augment.json",
		Augment: dataset.AugmentItem{
			Name:        "Set Augment: Example Set Name",
			AugmentType: "Colorless",
			MinLevel:    &level,
			SetBonus:    []dataset.SetBonusOut{{Name: "Example Set Name"}},
			EffectsAdded: []dataset.PartialEnhancementOut{{
				Name: "Item Set: Example Set Name",
			}},
		},
	}}}

	augments, _, err := transformAugments(context.Background(), master)
	if err != nil {
		t.Fatal(err)
	}
	if len(augments) != 1 || len(augments[0].Effects) != 1 || augments[0].Effects[0].DisplayName != "Item Set: Example Set Name" {
		t.Fatalf("compatibility projection = %#v, want exactly one item-set effect", augments)
	}
}

func TestTransformAugmentsRejectsDuplicateItemSetMembership(t *testing.T) {
	level := 1
	master := dataset.Master{Augments: []dataset.AugmentRecord{{
		File: "augment.json",
		Augment: dataset.AugmentItem{
			Name:        "Duplicate Set Membership",
			AugmentType: "Colorless",
			MinLevel:    &level,
			SetBonus: []dataset.SetBonusOut{
				{Name: "Example Set Name"},
				{Name: "Example Set Name"},
			},
		},
	}}}

	_, _, err := transformAugments(context.Background(), master)
	if err == nil || !strings.Contains(err.Error(), `repeats semantic effect "Item Set: Example Set Name"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestTransformAugmentsScopesEffectIDsToAugments(t *testing.T) {
	level := 30
	master := dataset.Master{Augments: []dataset.AugmentRecord{
		{File: "first.json", Augment: dataset.AugmentItem{Name: "First Fire Augment", AugmentType: "Red", MinLevel: &level, EffectsAdded: []dataset.PartialEnhancementOut{{Name: "Fire Absorption", Bonus: "Enhancement", Modifier: float64(10)}}}},
		{File: "second.json", Augment: dataset.AugmentItem{Name: "Second Fire Augment", AugmentType: "Red", MinLevel: &level, EffectsAdded: []dataset.PartialEnhancementOut{{Name: "Fire Absorption", Bonus: "Enhancement", Modifier: float64(20)}}}},
	}}

	first, _, err := transformAugments(context.Background(), master)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := transformAugments(context.Background(), master)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("transformAugments() is not deterministic: %#v != %#v", first, second)
	}
	if len(first) != 2 || len(first[0].Effects) != 1 || len(first[1].Effects) != 1 {
		t.Fatalf("augments = %#v", first)
	}
	firstEffect, secondEffect := first[0].Effects[0], first[1].Effects[0]
	if firstEffect.ID == secondEffect.ID {
		t.Fatalf("shared semantic augment effects use the same ID %q", firstEffect.ID)
	}
	wantModifierValues := map[string]float64{"First Fire Augment": 1000, "Second Fire Augment": 2000}
	for _, augment := range first {
		effect := augment.Effects[0]
		if want := effectID(augment.ID, "Enhancement", "Fire Absorption"); effect.ID != want {
			t.Fatalf("augment %q effect ID = %q, want %q", augment.DisplayName, effect.ID, want)
		}
		if effect.Modifier.Value != wantModifierValues[augment.DisplayName] {
			t.Fatalf("augment %q modifier = %#v", augment.DisplayName, effect.Modifier)
		}
	}
}
