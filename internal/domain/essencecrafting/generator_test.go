package essencecrafting

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestGenerateAndWrite(t *testing.T) {
	t.Parallel()
	raw := []RawEnhancement{{
		Name: "Mighty Effect", Group: "Ability Scores", Prefix: []string{"Ring", "Ring"},
		Enchantments: []RawEnchantment{{Name: "Strength", Bonus: "Enhancement"}}, Stat: []any{2.0, 4.0},
	}}
	output := Generate(raw)
	if len(output.Effects) != 1 || output.Effects[0].ID != "mighty-effect" {
		t.Fatalf("effects = %#v", output.Effects)
	}
	if len(output.PlannerEntries) != 1 || output.PlannerEntries[0].SourceType != "essence" {
		t.Fatalf("planner entries = %#v", output.PlannerEntries)
	}
	root := t.TempDir()
	if err := Write(root, output); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"effects.json", "enchantments.json", "placements.json", "tiers.json", "recipes.json", "display.json", "planner_entries.json", "indexes.json"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			t.Fatalf("%s does not end in newline", name)
		}
		compact, err := dataset.CompactJSON(data)
		if err != nil {
			t.Fatalf("%s is not valid JSON: %v", name, err)
		}
		if !bytes.Equal(compact, data) {
			t.Fatalf("%s is not minified", name)
		}
	}
}
