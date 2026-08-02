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
