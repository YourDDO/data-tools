package dinosaurbone

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestSelectionIncludesAccessoryAndWeaponSlotsAndEligibleAugments(t *testing.T) {
	t.Parallel()
	master := dataset.Master{
		Items: []dataset.ItemRecord{
			{File: "ring.json", Item: dataset.ItemData{PageTitle: "Bone Ring", Name: "Bone Ring", Type: "Ring", Augments: []dataset.AugmentItem{{AugmentType: "Isle of Dread: Claw Slot (Accessory)"}}}},
			{File: "sword.json", Item: dataset.ItemData{PageTitle: "Bone Sword", Name: "Bone Sword", Type: "Long Sword", Augments: []dataset.AugmentItem{{AugmentType: "Isle of Dread: Fang Slot (Weapon)"}}}},
			{File: "other.json", Item: dataset.ItemData{PageTitle: "Other", Name: "Other", DropLocations: []dataset.DropSourceData{{SourceType: "Dinosaur Bone Crafting"}}}},
		},
		Augments: []dataset.AugmentRecord{
			{File: "augment.json", Augment: dataset.AugmentItem{Name: "Claw Effect", AugmentType: "Isle of Dread: Claw (Accessory)"}},
			{File: "augment.json", Augment: dataset.AugmentItem{Name: "Ruby", AugmentType: "Red"}},
		},
	}
	items, err := selectItems(context.Background(), master.Items)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Name != "Bone Ring" || items[1].Name != "Bone Sword" {
		t.Fatalf("selected items = %#v", items)
	}
	augments, err := selectAugments(context.Background(), master.Augments)
	if err != nil {
		t.Fatal(err)
	}
	if len(augments) != 1 || augments[0].Name != "Claw Effect" {
		t.Fatalf("selected augments = %#v", augments)
	}
}

func TestGenerateCopiesExactMasterItem(t *testing.T) {
	t.Parallel()
	want := json.RawMessage(`{"pageTitle":"Bone Artifact","name":"Bone Artifact","type":"Ring","artifactType":"Minor","artifact":{"tier":"mythic"},"setBonus":[{"name":"The Legendary Dread Isle's Curse"}],"futureField":"preserved","augments":[{"augmentType":"Isle of Dread: Scale Slot (Accessory)"}],"enchantments":[]}`)
	var typed dataset.ItemData
	if err := json.Unmarshal(want, &typed); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	_, err := New().Generate(context.Background(), dataset.Master{Items: []dataset.ItemRecord{{
		File: "ring.json", Item: typed, Raw: want,
	}}}, root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, Name, "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got []json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !bytes.Equal(got[0], want) {
		t.Fatalf("Dinosaur Bone item = %s, want exact master item %s", got[0], want)
	}
}
