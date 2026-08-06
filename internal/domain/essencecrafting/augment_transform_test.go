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
		effect, err := transformAugmentEffect(input)
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

	_, _, _, err := transformAugments(context.Background(), master)
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
	wantID := opaqueID("effect", "\x00Item Set: Example Set Name")
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

	augments, _, _, err := transformAugments(context.Background(), master)
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

	_, _, _, err := transformAugments(context.Background(), master)
	if err == nil || !strings.Contains(err.Error(), `repeats semantic effect "Item Set: Example Set Name"`) {
		t.Fatalf("error = %v", err)
	}
}
