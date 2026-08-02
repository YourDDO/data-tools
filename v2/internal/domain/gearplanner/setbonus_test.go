package gearplanner

import (
	"testing"

	"yourddo-data-tools/v2/internal/dataset"
)

func TestGenerateSetBonusIndexSortsAndDeduplicates(t *testing.T) {
	t.Parallel()
	items := []dataset.ItemData{
		{Name: "Later", MinLevel: "20 / 30", SetBonus: []dataset.SetBonusOut{{Name: "Set"}}},
		{Name: "Earlier", MinLevel: "5", SetBonus: []dataset.SetBonusOut{{Name: "Set"}, {Name: "Set"}}},
	}
	index := GenerateSetBonusIndex(items)
	if len(index["Set"]) != 2 || index["Set"][0].Name != "Earlier" || index["Set"][1].MinLevel != 20 {
		t.Fatalf("index = %#v", index)
	}
}

func TestGenerateSetBonusIndexUsesFiligreePageTitleToPreventDataLoss(t *testing.T) {
	t.Parallel()
	records := []dataset.ItemRecord{
		{Category: "Filigrees", Item: dataset.ItemData{
			PageTitle: "Electrocution: PRR", Name: "Electrocution: PRR", Type: "Common",
			SetBonus: []dataset.SetBonusOut{{Name: "Electrocution"}},
		}},
		{Category: "Filigrees", Item: dataset.ItemData{
			PageTitle: "Electrocution: PRR (Rare)", Name: "Electrocution: PRR", Type: "Rare",
			SetBonus: []dataset.SetBonusOut{{Name: "Electrocution"}},
		}},
		{Category: "Filigrees", Item: dataset.ItemData{
			PageTitle: "The Enlightened Step: Dexterity (Rare)", Name: "The Enlightened Step: Constitution (Rare)", Type: "Rare",
			SetBonus: []dataset.SetBonusOut{{Name: "The Enlightened Step"}},
		}},
	}
	index := GenerateSetBonusIndexFromRecords(records)
	if got := index["Electrocution"]; len(got) != 2 || got[0].Name != "Electrocution: PRR" || got[1].Name != "Electrocution: PRR (Rare)" {
		t.Fatalf("Electrocution index = %#v", got)
	}
	if got := index["The Enlightened Step"]; len(got) != 1 || got[0].Name != "The Enlightened Step: Dexterity (Rare)" {
		t.Fatalf("Enlightened Step index = %#v", got)
	}
}

func TestGenerateSetBonusIndexUsesPageTitlesForSameNamedLevelVariants(t *testing.T) {
	t.Parallel()
	records := []dataset.ItemRecord{
		{Category: "Helmet", Item: dataset.ItemData{
			PageTitle: "Helm of the Black Dragon (Level 14)", Name: "Helm of the Black Dragon", MinLevel: "14",
			SetBonus: []dataset.SetBonusOut{{Name: "Draconic Ferocity"}},
		}},
		{Category: "Helmet", Item: dataset.ItemData{
			PageTitle: "Helm of the Black Dragon (Level 25)", Name: "Helm of the Black Dragon", MinLevel: "25",
			SetBonus: []dataset.SetBonusOut{{Name: "Draconic Ferocity"}},
		}},
		{Category: "Helmet", Item: dataset.ItemData{
			PageTitle: "Helm of the Black Dragon (Level 31)", Name: "Helm of the Black Dragon", MinLevel: "31",
			SetBonus: []dataset.SetBonusOut{{Name: "Draconic Ferocity"}},
		}},
	}

	got := GenerateSetBonusIndexFromRecords(records)["Draconic Ferocity"]
	want := []SetItem{
		{Name: "Helm of the Black Dragon (Level 14)", MinLevel: 14},
		{Name: "Helm of the Black Dragon (Level 25)", MinLevel: 25},
		{Name: "Helm of the Black Dragon (Level 31)", MinLevel: 31},
	}
	if len(got) != len(want) {
		t.Fatalf("Draconic Ferocity index = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Draconic Ferocity index = %#v, want %#v", got, want)
		}
	}
}
