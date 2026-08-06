package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"yourddo-data-tools/internal/compendium/cleanup"
	"yourddo-data-tools/internal/dataset"
)

func TestParseAugmentRecordEssenceOfDarkDiversion(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "essence_of_dark_diversion.wiki"))
	if err != nil {
		t.Fatal(err)
	}
	fields, err := parseTemplateFields(cleanup.CleanRawContent(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"name":         "Essence of Dark Diversion",
		"color":        "Yellow",
		"minlevel":     "30",
		"binding":      "btaoa",
		"bagtype":      "Augment",
		"description":  "This topaz carries the soul of Dark Diversion.",
		"droplocation": "{{CraftedAugment|Soulforge|Dark Diversion}}",
		"enchantments": "{{Incite|-20|All||Occultation}}",
		"icon":         "Augment Yellow 4",
		"weight":       "0.01",
		"basevalue":    "{{Price|500}}",
		"update":       "48",
	} {
		if got := fields[key]; got != want {
			t.Fatalf("core field %q = %q, want %q", key, got, want)
		}
	}

	augment, err := ParseAugmentRecord("Essence of Dark Diversion", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := []dataset.PartialEnhancementOut{
		{Name: "Melee Threat", Modifier: -0.2, Bonus: ""},
		{Name: "Ranged Threat", Modifier: -0.2, Bonus: ""},
		{Name: "Spell Threat", Modifier: -0.2, Bonus: ""},
	}
	if !reflect.DeepEqual(augment.EffectsAdded, want) {
		t.Fatalf("EffectsAdded = %#v, want %#v", augment.EffectsAdded, want)
	}
	seen := map[string]struct{}{}
	for index, effect := range augment.EffectsAdded {
		identity := effect.Bonus + "\x00" + effect.Name
		if _, exists := seen[identity]; exists {
			t.Fatalf("effects[%d] repeats semantic identity %q", index, identity)
		}
		seen[identity] = struct{}{}
	}
}
