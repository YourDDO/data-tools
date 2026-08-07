package essencecrafting

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
)

func TestGenerateUpdate81Domain(t *testing.T) {
	manualRoot := filepath.Join("..", "..", "..", "inputs", "manual")
	master := fixtureMaster()
	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	first, err := New().GenerateWithManual(context.Background(), master, manualRoot, firstRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New().GenerateWithManual(context.Background(), master, manualRoot, secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("generation metadata differs\nfirst: %#v\nsecond: %#v", first, second)
	}
	firstBytes, err := os.ReadFile(filepath.Join(firstRoot, Name, "essence-crafting.json"))
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(filepath.Join(secondRoot, Name, "essence-crafting.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("generation is not byte deterministic")
	}
	if firstBytes[len(firstBytes)-1] != '\n' {
		t.Fatal("output does not terminate with newline")
	}
	var output contracts.EssenceCraftingDomain
	if err := json.Unmarshal(firstBytes, &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Enhancements) != 368 || len(output.Recipes) != 808 || len(output.MinimumLevelShards) != 36 || len(output.Augments) != 7 {
		t.Fatalf("record counts = enhancements:%d recipes:%d ML:%d augments:%d", len(output.Enhancements), len(output.Recipes), len(output.MinimumLevelShards), len(output.Augments))
	}
	byName := map[string]contracts.EssenceCraftingEnhancement{}
	for _, enhancement := range output.Enhancements {
		byName[enhancement.DisplayName] = enhancement
	}
	honed := byName["Honed"]
	if honed.ID == "" || honed.MinimumItemLevel != 20 || len(honed.Placements) != 1 || honed.Placements[0].Position != "prefix" || len(honed.Effects) != 3 {
		t.Fatalf("Honed = %#v", honed)
	}
	focus := byName["Insightful Spell Focus Mastery"]
	if len(focus.Placements) != 1 || focus.Placements[0].Position != "extra" || !reflect.DeepEqual(focus.Placements[0].ItemCategoryIDs, []string{"head", "trinket"}) || len(focus.Effects) != 7 {
		t.Fatalf("Insightful Spell Focus Mastery = %#v", focus)
	}
	for _, name := range []string{"Insightful Combustion", "Insightful Corrosion", "Insightful Devotion", "Insightful Glaciation", "Insightful Impulse", "Insightful Magnetism", "Insightful Nullification", "Insightful Radiance", "Insightful Reconstruction", "Insightful Resonance"} {
		found := false
		for _, placement := range byName[name].Placements {
			if placement.Position == "extra" && contains(placement.ItemCategoryIDs, "ring") {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s is missing extra/ring", name)
		}
	}
	if _, exists := byName["Silver Flame [Combined]"]; !exists {
		t.Fatal("corrected Silver Flame record is missing")
	}
	for _, recipe := range output.Recipes {
		if recipe.Binding == "bound" && strings.HasPrefix(recipe.ID, "recipe-source-") {
			for _, requirement := range recipe.Requirements {
				if requirement.IngredientID == opaqueID("ingredient", purifiedFragment) && isSplitBoundRecipe(recipe.ID, output.Enhancements) {
					t.Fatalf("%s has Purified in bound split-prefix recipe", recipe.ID)
				}
			}
		}
	}
	for _, augment := range output.Augments {
		if augment.AugmentTypeID != "red" && augment.AugmentTypeID != "blue" && augment.AugmentTypeID != "yellow" && augment.AugmentTypeID != "green" && augment.AugmentTypeID != "purple" && augment.AugmentTypeID != "orange" && augment.AugmentTypeID != "colorless" {
			t.Fatalf("unexpected augment type %q", augment.AugmentTypeID)
		}
	}
	effectOwners := map[string]string{}
	enhancementEffectCount, augmentEffectCount := 0, 0
	fireAbsorptionIDs := map[string]string{}
	for _, enhancement := range output.Enhancements {
		for _, effect := range enhancement.Effects {
			enhancementEffectCount++
			if previous, exists := effectOwners[effect.ID]; exists {
				t.Fatalf("duplicate nested effect ID %q in %s and enhancement %q", effect.ID, previous, enhancement.DisplayName)
			}
			effectOwners[effect.ID] = "enhancement " + enhancement.DisplayName
			if effect.DisplayName == "Fire Absorption" && contains([]string{"Flame Attuned", "Fire Absorption", "Firewarded", "Flame Absorbing"}, enhancement.DisplayName) {
				fireAbsorptionIDs[enhancement.DisplayName] = effect.ID
			}
		}
	}
	for _, augment := range output.Augments {
		for _, effect := range augment.Effects {
			augmentEffectCount++
			if previous, exists := effectOwners[effect.ID]; exists {
				t.Fatalf("duplicate nested effect ID %q in %s and augment %q", effect.ID, previous, augment.DisplayName)
			}
			effectOwners[effect.ID] = "augment " + augment.DisplayName
		}
	}
	if len(fireAbsorptionIDs) != 4 {
		t.Fatalf("Fire Absorption IDs for expected enhancements = %v, want four distinct IDs", fireAbsorptionIDs)
	}
	seenFireAbsorptionIDs := map[string]string{}
	for _, name := range []string{"Flame Attuned", "Fire Absorption", "Firewarded", "Flame Absorbing"} {
		if previous, exists := seenFireAbsorptionIDs[fireAbsorptionIDs[name]]; exists {
			t.Fatalf("%s and %s share Fire Absorption ID %q", previous, name, fireAbsorptionIDs[name])
		}
		seenFireAbsorptionIDs[fireAbsorptionIDs[name]] = name
		t.Logf("%s Fire Absorption ID=%s", name, fireAbsorptionIDs[name])
	}
	t.Logf("generated nested effects: enhancements=%d augments=%d duplicates=0", enhancementEffectCount, augmentEffectCount)
	if len(output.Ingredients) == 0 {
		t.Fatal("ingredient catalog is empty")
	}
}

func TestRulesUseAugmentCompatibility(t *testing.T) {
	want := map[string][]string{
		"blue":      {"blue", "colorless"},
		"colorless": {"colorless"},
		"green":     {"blue", "colorless", "green", "yellow"},
		"orange":    {"colorless", "orange", "red", "yellow"},
		"purple":    {"blue", "colorless", "purple", "red"},
		"red":       {"colorless", "red"},
		"yellow":    {"colorless", "yellow"},
	}
	for _, slot := range rules().AugmentSlotTypes {
		if !reflect.DeepEqual(slot.AcceptsAugmentTypeIDs, want[slot.ID]) {
			t.Fatalf("%s accepts %v, want %v", slot.ID, slot.AcceptsAugmentTypeIDs, want[slot.ID])
		}
		delete(want, slot.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing slot rules: %v", want)
	}
}

func TestBuildRejectsInvalidInputs(t *testing.T) {
	base := sourceEnhancement{Name: "Test", MinimumLevel: 1, Prefix: []string{"Ring"}, Bound: &sourceRecipe{RecipeID: 1, Level: 1, Essence: 1}, Unbound: &sourceRecipe{RecipeID: 2, Level: 1, Essence: 1}, Effects: []sourceEffect{{Name: "Effect", Modifiers: fullScale(1)}}}
	tests := []struct {
		name   string
		mutate func(*sourceEnhancement)
		want   string
	}{
		{"missing unbound", func(v *sourceEnhancement) { v.Unbound = nil }, "bound and unbound"},
		{"unsupported position category", func(v *sourceEnhancement) { v.Prefix = []string{"Browser Ring"} }, "unsupported"},
		{"invalid quantity", func(v *sourceEnhancement) { v.Bound.Collectibles = []sourceRequirement{{Name: "Thing", Quantity: 0}} }, "invalid name or quantity"},
		{"missing scale", func(v *sourceEnhancement) { v.Effects[0].Modifiers = nil }, "missing modifier scale"},
		{"duplicate recipes", func(v *sourceEnhancement) { v.Unbound.RecipeID = 1 }, "shared"},
		{"duplicate semantic effects", func(v *sourceEnhancement) { v.Effects = append(v.Effects, v.Effects[0]) }, "repeats semantic effect"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneSource(t, base)
			test.mutate(&value)
			_, err := build(context.Background(), fixtureMaster(), []sourceEnhancement{value})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestBuildScopesEffectIDsToEnhancements(t *testing.T) {
	firstInput := sourceEnhancement{Name: "First Fire Enhancement", MinimumLevel: 1, Prefix: []string{"Ring"}, Bound: &sourceRecipe{RecipeID: 101, Level: 1, Essence: 1}, Unbound: &sourceRecipe{RecipeID: 102, Level: 1, Essence: 1}, Effects: []sourceEffect{{Name: "Fire Absorption", Bonus: "Enhancement", Modifiers: fullScale(1)}}}
	secondInput := sourceEnhancement{Name: "Second Fire Enhancement", MinimumLevel: 1, Prefix: []string{"Ring"}, Bound: &sourceRecipe{RecipeID: 103, Level: 1, Essence: 1}, Unbound: &sourceRecipe{RecipeID: 104, Level: 1, Essence: 1}, Effects: []sourceEffect{{Name: "Fire Absorption", Bonus: "Enhancement", Modifiers: fullScale(1)}}}
	for index := range secondInput.Effects[0].Modifiers {
		secondInput.Effects[0].Modifiers[index].Value = json.Number("2")
	}

	first, err := build(context.Background(), dataset.Master{}, []sourceEnhancement{firstInput, secondInput})
	if err != nil {
		t.Fatal(err)
	}
	second, err := build(context.Background(), dataset.Master{}, []sourceEnhancement{firstInput, secondInput})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("build is not deterministic")
	}
	if len(first.Enhancements) != 2 {
		t.Fatalf("enhancements = %#v", first.Enhancements)
	}
	firstEffect, secondEffect := first.Enhancements[0].Effects[0], first.Enhancements[1].Effects[0]
	if firstEffect.ID == secondEffect.ID {
		t.Fatalf("shared semantic enhancement effects use the same ID %q", firstEffect.ID)
	}
	wantModifierValues := map[string]float64{"First Fire Enhancement": 100, "Second Fire Enhancement": 200}
	for _, enhancement := range first.Enhancements {
		effect := enhancement.Effects[0]
		if want := effectID(enhancement.ID, "Enhancement", "Fire Absorption"); effect.ID != want {
			t.Fatalf("enhancement %q effect ID = %q, want %q", enhancement.DisplayName, effect.ID, want)
		}
		if effect.DisplayName != "Fire Absorption" || effect.Modifier == nil || effect.Modifier.Bands[0].Value != wantModifierValues[enhancement.DisplayName] {
			t.Fatalf("enhancement %q effect = %#v", enhancement.DisplayName, effect)
		}
	}
}

func fixtureMaster() dataset.Master {
	colors := []string{"Red", "Blue", "Yellow", "Green", "Purple", "Orange", "Colorless"}
	master := dataset.Master{}
	for _, color := range colors {
		level := 1
		master.Augments = append(master.Augments, dataset.AugmentRecord{File: "augments.json", Augment: dataset.AugmentItem{Name: color + " Fixture", AugmentType: color, MinLevel: &level, EffectsAdded: []dataset.PartialEnhancementOut{{Name: "Fixture " + color, Modifier: float64(1)}}}})
	}
	return master
}

func fullScale(minimum int) []sourceModifier {
	values := make([]sourceModifier, 0, maximumItemLevel-minimum+1)
	for level := minimum; level <= maximumItemLevel; level++ {
		values = append(values, sourceModifier{Level: level, Value: json.Number("1")})
	}
	return values
}
func cloneSource(t *testing.T, value sourceEnhancement) sourceEnhancement {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result sourceEnhancement
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func isSplitBoundRecipe(recipeID string, values []contracts.EssenceCraftingEnhancement) bool {
	for _, value := range values {
		if value.MinimumItemLevel == 20 && value.Recipes.BoundRecipeID == recipeID {
			return true
		}
	}
	return false
}
