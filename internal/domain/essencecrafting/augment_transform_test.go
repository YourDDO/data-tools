package essencecrafting

import (
	"context"
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
