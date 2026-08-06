package compendium

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"yourddo-data-tools/internal/compendium/parser"
	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain/essencecrafting"
)

func TestEssenceOfDarkDiversionCanonicalAndEssenceCraftingIntegration(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("parser", "testdata", "essence_of_dark_diversion.wiki"))
	if err != nil {
		t.Fatal(err)
	}

	// Start at the production-shaped parser entry point before exercising
	// canonical normalization and the domain generator.
	parsed, err := parser.ParseAugmentRecord("Essence of Dark Diversion", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	wantEffects := []dataset.PartialEnhancementOut{
		{Name: "Melee Threat", Modifier: -0.2},
		{Name: "Ranged Threat", Modifier: -0.2},
		{Name: "Spell Threat", Modifier: -0.2},
	}
	if !reflect.DeepEqual(parsed.EffectsAdded, wantEffects) {
		t.Fatalf("parsed EffectsAdded = %#v, want %#v", parsed.EffectsAdded, wantEffects)
	}

	generator, err := NewGenerator(fixtureSource{categories: map[string]map[string]string{
		"Augment": {"Essence of Dark Diversion": string(raw)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	masterRoot := filepath.Join(t.TempDir(), "master")
	canonical, err := generator.Generate(context.Background(), []string{"Augment"}, masterRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(canonical.Master.Augments) != 1 || !reflect.DeepEqual(canonical.Master.Augments[0].Augment.EffectsAdded, wantEffects) {
		t.Fatalf("canonical augment = %#v, want effects %#v", canonical.Master.Augments, wantEffects)
	}

	outputRoot := t.TempDir()
	if _, err := essencecrafting.New().GenerateWithManual(context.Background(), canonical.Master, filepath.Join("..", "..", "inputs", "manual"), outputRoot); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(outputRoot, essencecrafting.Name, "essence-crafting.json"))
	if err != nil {
		t.Fatal(err)
	}
	var output contracts.EssenceCraftingDomain
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	for _, augment := range output.Augments {
		if augment.DisplayName != "Essence of Dark Diversion" {
			continue
		}
		if len(augment.Effects) != 3 {
			t.Fatalf("generated effect count = %d, want 3", len(augment.Effects))
		}
		seenIDs := map[string]struct{}{}
		byName := map[string]contracts.EssenceCraftingEffect{}
		for _, effect := range augment.Effects {
			if _, exists := seenIDs[effect.ID]; exists {
				t.Fatalf("duplicate generated effect ID %q", effect.ID)
			}
			seenIDs[effect.ID] = struct{}{}
			byName[effect.DisplayName] = effect
		}
		for _, name := range []string{"Melee Threat", "Ranged Threat", "Spell Threat"} {
			effect, exists := byName[name]
			if !exists || effect.Modifier == nil || effect.Modifier.Kind != "fixed" || effect.Modifier.Unit != "percent" || effect.Modifier.Value != -20.0 {
				t.Fatalf("generated %q = %#v, want fixed -20 percent", name, effect)
			}
		}
		return
	}
	t.Fatal("Essence of Dark Diversion is missing from generated Essence Crafting data")
}
