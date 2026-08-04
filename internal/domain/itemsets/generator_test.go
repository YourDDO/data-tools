package itemsets

import (
	"encoding/json"
	"reflect"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestGenerateSetBonusIndexFromMasterIncludesItemsAndAugments(t *testing.T) {
	t.Parallel()
	level := 30
	master := dataset.Master{
		Items: []dataset.ItemRecord{{Item: dataset.ItemData{
			PageTitle: "Set Helm", Name: "Set Helm", MinLevel: "20 / 30",
			SetBonus: []dataset.SetBonusOut{{Name: "Shared Set"}, {Name: "Shared Set"}, {Name: "True"}},
		}}},
		Augments: []dataset.AugmentRecord{{Augment: dataset.AugmentItem{
			Name: "Set Augment", AugmentType: "Colorless", MinLevel: &level,
			SetBonus: []dataset.SetBonusOut{{Name: "Shared Set"}, {Name: "Augment-Only Set"}},
		}}},
	}
	index := GenerateSetBonusIndexFromMaster(master)
	if got, want := index["Shared Set"], []SetItem{{Name: "Set Helm", MinLevel: 20}, {Name: "Set Augment", MinLevel: 30}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("shared set = %#v, want %#v", got, want)
	}
	if got, want := index["Augment-Only Set"], []SetItem{{Name: "Set Augment", MinLevel: 30}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("augment-only set = %#v, want %#v", got, want)
	}
	if _, exists := index["True"]; exists {
		t.Fatalf("template control was emitted as a set: %#v", index["True"])
	}
}

func TestFiligreeSetNamesSortsAndDeduplicates(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal([]dataset.FiligreeSet{{Name: "Later"}, {Name: "Earlier"}, {Name: "Later"}})
	if err != nil {
		t.Fatal(err)
	}
	master := dataset.Master{Files: []dataset.CanonicalFile{{
		MasterFile: dataset.MasterFile{Kind: "filigree-sets", Path: "filigreeSets.json"}, Data: data,
	}}}
	names, err := FiligreeSetNames(master)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Earlier", "Later"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}
