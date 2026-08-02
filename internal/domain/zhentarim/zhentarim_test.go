package zhentarim

import (
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestGenerateRequiresAttunedBaseAndUpgrade(t *testing.T) {
	t.Parallel()
	items := []dataset.ItemData{
		{PageTitle: "Item", Name: "Item", Enchantments: []dataset.Enchantment{{Name: "Zhentarim Attuned"}}},
		{PageTitle: "Item (Upgraded)", Name: "Item", Enchantments: []dataset.Enchantment{{Name: "Power"}}},
	}
	output, warnings := Generate(items)
	if len(warnings) != 0 || len(output) != 1 || output[0].EffectsAdded[0].Name != "Power" {
		t.Fatalf("output = %#v, warnings = %#v", output, warnings)
	}
}
