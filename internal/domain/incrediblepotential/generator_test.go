package incrediblepotential

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain/itemlist"
)

func TestGenerateSelectsIncrediblePotentialItems(t *testing.T) {
	t.Parallel()
	master := dataset.Master{Items: []dataset.ItemRecord{
		{File: "rings.json", Item: dataset.ItemData{PageTitle: "Potential Ring", Name: "Potential Ring", Type: "Ring", MinLevel: "18", Enchantments: []dataset.Enchantment{{Name: "Incredible Potential"}}}},
		{File: "rings.json", Item: dataset.ItemData{PageTitle: "Other Ring", Name: "Other Ring", Type: "Ring", MinLevel: "18", Enchantments: []dataset.Enchantment{{Name: "Other Effect"}}}},
	}}
	root := t.TempDir()
	result, err := New().Generate(context.Background(), master, root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Domain != Name || len(result.Files) != 1 || result.Files[0].Path != "incredible-potential/items.json" {
		t.Fatalf("result = %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(root, "incredible-potential", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items []itemlist.Item
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].PageTitle != "Potential Ring" {
		t.Fatalf("items = %#v", items)
	}
}
