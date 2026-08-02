package dinosaurbone

import (
	"context"
	"testing"

	"yourddo-data-tools/v2/internal/dataset"
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
