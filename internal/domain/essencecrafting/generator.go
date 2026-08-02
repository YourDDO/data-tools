package essencecrafting

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"yourddo-data-tools/internal/dataset"
)

type Outputs struct {
	Effects        []EffectDefinition
	Enchantments   []EffectEnchantment
	Placements     []EffectPlacement
	Tiers          []EffectTiers
	Recipes        []EffectRecipe
	Display        []EffectDisplay
	PlannerEntries []PlannerEntry
	Indexes        Indexes
}

func Load(path string) ([]RawEnhancement, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Essence Crafting input %s: %w", path, err)
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("decode Essence Crafting input %s as an array: %w", path, err)
	}
	result := make([]RawEnhancement, 0, len(messages))
	for index, message := range messages {
		var enhancement RawEnhancement
		if err := json.Unmarshal(message, &enhancement); err != nil {
			return nil, fmt.Errorf("decode Essence Crafting enhancement at index %d: %w", index, err)
		}
		result = append(result, enhancement)
	}
	return result, nil
}

func Generate(raw []RawEnhancement) Outputs {
	effects, enchantments, placements, tiers, recipes, display, plannerEntries := transform(raw)
	return Outputs{
		Effects:        effects,
		Enchantments:   enchantments,
		Placements:     placements,
		Tiers:          tiers,
		Recipes:        recipes,
		Display:        display,
		PlannerEntries: plannerEntries,
		Indexes:        buildIndexes(plannerEntries),
	}
}

func Write(outputRoot string, outputs Outputs) error {
	files := []struct {
		name  string
		value any
	}{
		{"effects.json", outputs.Effects},
		{"enchantments.json", outputs.Enchantments},
		{"placements.json", outputs.Placements},
		{"tiers.json", outputs.Tiers},
		{"recipes.json", outputs.Recipes},
		{"display.json", outputs.Display},
		{"planner_entries.json", outputs.PlannerEntries},
		{"indexes.json", outputs.Indexes},
	}
	for _, file := range files {
		if err := dataset.WriteJSON(filepath.Join(outputRoot, file.name), file.value); err != nil {
			return err
		}
	}
	return nil
}
