package manual

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// These types deliberately describe the checked-in manual payload rather than
// the dormant essencecrafting domain input.  The manual payload is the current
// canonical source consumed by the release pipeline.
type essenceCraftingV2Record struct {
	Name         string                    `json:"name"`
	MinItemLevel int                       `json:"minItemLevel"`
	Bound        essenceCraftingV2Recipe   `json:"bound"`
	Unbound      essenceCraftingV2Recipe   `json:"unbound"`
	Prefix       []string                  `json:"prefix"`
	Suffix       []string                  `json:"suffix"`
	Extra        []string                  `json:"extra"`
	Enchantments []essenceCraftingV2Effect `json:"enchantments"`
}

type essenceCraftingV2Recipe struct {
	RecipeID     int                            `json:"recipeId"`
	Level        int                            `json:"level"`
	Essence      int                            `json:"essence"`
	Collectibles []essenceCraftingV2Collectible `json:"collectible"`
}

type essenceCraftingV2Collectible struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type essenceCraftingV2Effect struct {
	Name      string                      `json:"name"`
	Modifiers []essenceCraftingV2Modifier `json:"modifiers"`
}

type essenceCraftingV2Modifier struct {
	Level int `json:"level"`
}

func TestEssenceCraftingV2Update81And81_1Source(t *testing.T) {
	records := loadEssenceCraftingV2(t)

	byName := make(map[string]essenceCraftingV2Record, len(records))
	recipeIDs := make(map[int]string, len(records)*2)
	splitPrefix := make([]essenceCraftingV2Record, 0, 107)
	legacyCollectibles := make(map[string]struct{})
	splitCollectibles := make(map[string]struct{})
	pairs := make(map[[2]int]int)

	for _, record := range records {
		if record.Name == "" {
			t.Fatal("record with an empty name")
		}
		if _, exists := byName[record.Name]; exists {
			t.Fatalf("duplicate enhancement identity %q", record.Name)
		}
		byName[record.Name] = record

		for binding, recipe := range map[string]essenceCraftingV2Recipe{"bound": record.Bound, "unbound": record.Unbound} {
			if recipe.RecipeID <= 0 || recipe.Level <= 0 || recipe.Essence <= 0 {
				t.Fatalf("%s %s recipe is incomplete: %#v", record.Name, binding, recipe)
			}
			if recipe.Level > 500 {
				t.Fatalf("%s %s recipe level %d exceeds the Update 81 crafting maximum", record.Name, binding, recipe.Level)
			}
			if owner, exists := recipeIDs[recipe.RecipeID]; exists {
				t.Fatalf("recipe ID %d is shared by %s and %s", recipe.RecipeID, owner, record.Name)
			}
			recipeIDs[recipe.RecipeID] = record.Name
			for _, collectible := range recipe.Collectibles {
				if collectible.Name == "" || collectible.Quantity <= 0 {
					t.Fatalf("%s %s has an unresolved collectible reference: %#v", record.Name, binding, collectible)
				}
			}
		}

		for _, effect := range record.Enchantments {
			if effect.Name == "" {
				t.Fatalf("%s has an unnamed effect", record.Name)
			}
			if len(effect.Modifiers) == 0 {
				if record.Name != "Necromantic" || effect.Name != "Deathblock" {
					t.Fatalf("%s/%s has no level scaling", record.Name, effect.Name)
				}
				continue
			}
			seenLevels := make(map[int]struct{}, len(effect.Modifiers))
			for _, modifier := range effect.Modifiers {
				if modifier.Level < record.MinItemLevel || modifier.Level > 36 {
					t.Fatalf("%s/%s has modifier level %d outside ML%d-36", record.Name, effect.Name, modifier.Level, record.MinItemLevel)
				}
				if _, exists := seenLevels[modifier.Level]; exists {
					t.Fatalf("%s/%s repeats modifier level %d", record.Name, effect.Name, modifier.Level)
				}
				seenLevels[modifier.Level] = struct{}{}
			}
			if _, exists := seenLevels[record.MinItemLevel]; !exists {
				t.Fatalf("%s/%s does not start at its minimum item level %d", record.Name, effect.Name, record.MinItemLevel)
			}
			if _, exists := seenLevels[36]; !exists {
				t.Fatalf("%s/%s does not support item level 36", record.Name, effect.Name)
			}
		}

		if record.MinItemLevel == 20 {
			splitPrefix = append(splitPrefix, record)
			pairs[[2]int{record.Bound.Level, record.Unbound.Level}]++
			for _, recipe := range []essenceCraftingV2Recipe{record.Bound, record.Unbound} {
				for _, collectible := range recipe.Collectibles {
					splitCollectibles[collectible.Name] = struct{}{}
				}
			}
		} else {
			for _, recipe := range []essenceCraftingV2Recipe{record.Bound, record.Unbound} {
				for _, collectible := range recipe.Collectibles {
					legacyCollectibles[collectible.Name] = struct{}{}
				}
			}
		}
	}

	if len(records) != 368 || len(recipeIDs) != 736 {
		t.Fatalf("records/recipe IDs = %d/%d, want 368/736", len(records), len(recipeIDs))
	}
	if len(splitPrefix) != 107 {
		t.Fatalf("ML20 split-prefix records = %d, want 107", len(splitPrefix))
	}
	if got := pairs[[2]int{400, 450}]; got != 54 {
		t.Fatalf("400/450 recipe pairs = %d, want 54", got)
	}
	if got := pairs[[2]int{425, 475}]; got != 53 {
		t.Fatalf("425/475 recipe pairs = %d, want 53", got)
	}

	t.Run("split prefix placement and Update 81.1 costs", func(t *testing.T) {
		for _, record := range splitPrefix {
			if len(record.Prefix) == 0 || len(record.Suffix) != 0 || len(record.Extra) != 0 {
				t.Fatalf("%s placement = prefix:%v suffix:%v extra:%v, want prefix-only", record.Name, record.Prefix, record.Suffix, record.Extra)
			}
			if hasCollectible(record.Bound, "Purified Eberron Dragonshard Fragment") {
				t.Fatalf("%s bound split-prefix recipe still has Purified Eberron Dragonshard Fragment", record.Name)
			}
			if got := collectibleQuantity(record.Unbound, "Purified Eberron Dragonshard Fragment"); got != 15 {
				t.Fatalf("%s unbound Purified quantity = %d, want 15", record.Name, got)
			}
		}

		honed := byName["Honed"]
		if len(honed.Enchantments) != 3 || honed.Enchantments[0].Name != "Melee Attack Speed" || honed.Enchantments[1].Name != "Movement Speed" || honed.Enchantments[2].Name != "Vorpal" {
			t.Fatalf("Honed must remain one split-prefix choice with both effects, got %#v", honed.Enchantments)
		}
	})

	t.Run("new collectible references", func(t *testing.T) {
		want := []string{
			"Annotated Manuscript", "Chunk of Raw Amber", "Creeping Vine", "Deciphered Riddle", "Envelope of Pyrrhic Pollen",
			"Etched Bone Shard", "Forgotten Folio", "Mystical Band", "Mystical Bottle", "Mystical Dried Fish", "Mystical Goblet",
			"Mystical Plant", "Mystical Urn", "Mystical Vessel", "Ornate Scroll Case", "Packet of Golden Powder", "Poisoned Thorn",
			"Silver Amulet", "Silver Bell",
		}
		got := make([]string, 0, len(splitCollectibles))
		for name := range splitCollectibles {
			if _, exists := legacyCollectibles[name]; !exists {
				got = append(got, name)
			}
		}
		sort.Strings(got)
		if !equalStrings(got, want) {
			t.Fatalf("Update 81 collectible references = %v, want %v", got, want)
		}
	})

	t.Run("corrected enhancement name", func(t *testing.T) {
		if _, exists := byName["Silver Flame (New)"]; exists {
			t.Fatal("stale Silver Flame (New) record is still selectable")
		}
		if _, exists := byName["Silver Flame [Combined]"]; !exists {
			t.Fatal("corrected Silver Flame [Combined] record is missing")
		}
	})

	t.Run("Update 81 extra placements", func(t *testing.T) {
		for _, name := range []string{
			"Insightful Combustion", "Insightful Corrosion", "Insightful Devotion", "Insightful Glaciation", "Insightful Impulse",
			"Insightful Magnetism", "Insightful Nullification", "Insightful Radiance", "Insightful Reconstruction", "Insightful Resonance",
		} {
			record, exists := byName[name]
			if !exists || !contains(record.Extra, "Ring") {
				t.Fatalf("%s must be available as an Extra on Ring, got %#v", name, record.Extra)
			}
		}

		focus := byName["Insightful Spell Focus Mastery"]
		if !equalStrings(focus.Extra, []string{"Head", "Trinket"}) || focus.Bound.RecipeID != 2030227489 || focus.Unbound.RecipeID != 2030227925 {
			t.Fatalf("Insightful Spell Focus Mastery placement/recipes = %#v", focus)
		}
		wantEffects := []string{
			"Abjuration Spell Focus", "Conjuration Spell Focus", "Enchantment Spell Focus", "Evocation Spell Focus",
			"Illusion Spell Focus", "Necromancy Spell Focus", "Transmutation Spell Focus",
		}
		gotEffects := make([]string, 0, len(focus.Enchantments))
		for _, effect := range focus.Enchantments {
			gotEffects = append(gotEffects, effect.Name)
		}
		if !equalStrings(gotEffects, wantEffects) {
			t.Fatalf("Insightful Spell Focus Mastery effects = %v, want %v", gotEffects, wantEffects)
		}
	})
}

func loadEssenceCraftingV2(t *testing.T) []essenceCraftingV2Record {
	t.Helper()
	path := filepath.Join("..", "..", "inputs", "manual", "essenceCrafting.v2.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read canonical Essence Crafting source %s: %v", path, err)
	}
	var records []essenceCraftingV2Record
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("decode canonical Essence Crafting source: %v", err)
	}
	return records
}

func hasCollectible(recipe essenceCraftingV2Recipe, name string) bool {
	return collectibleQuantity(recipe, name) > 0
}

func collectibleQuantity(recipe essenceCraftingV2Recipe, name string) int {
	for _, collectible := range recipe.Collectibles {
		if collectible.Name == name {
			return collectible.Quantity
		}
	}
	return 0
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
