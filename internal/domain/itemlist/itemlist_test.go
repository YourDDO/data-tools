package itemlist

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestGenerateCopiesCompleteCanonicalItem(t *testing.T) {
	t.Parallel()
	want := dataset.ItemData{
		PageTitle:    "Complete Item",
		Name:         "Complete Item",
		Type:         "Trinket",
		Description:  "Every canonical field belongs in a domain list.",
		MinLevel:     "30",
		ArtifactType: "Minor Artifact",
		SetBonus:     []dataset.SetBonusOut{{Name: "Complete Set"}},
		Augments:     []dataset.AugmentItem{{AugmentType: "Yellow"}},
		Enchantments: []dataset.Enchantment{{Name: "Domain Marker"}},
		Update:       "99",
		Icon:         "Complete Item",
	}

	root := t.TempDir()
	_, err := New("complete-items", func(dataset.ItemData) bool { return true }).Generate(
		context.Background(),
		dataset.Master{Items: []dataset.ItemRecord{{File: "trinkets.json", Item: want}}},
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "complete-items", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got []dataset.ItemData
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Fatalf("generated items = %#v, want %#v", got, []dataset.ItemData{want})
	}
}
