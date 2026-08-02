package fountain

import (
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestGenerateSelectsBaseAndUpgradedVariants(t *testing.T) {
	t.Parallel()
	items := []dataset.ItemData{
		{PageTitle: "Test Item (Pre U50)", Name: "Test Item", Enchantments: []dataset.Enchantment{{Name: "Old"}}},
		{PageTitle: "Test Item", Name: "Test Item", Enchantments: []dataset.Enchantment{{Name: "Base"}, {Name: "Upgradeable Item"}}},
		{PageTitle: "Test Item (Upgraded)", Name: "Test Item", Enchantments: []dataset.Enchantment{{Name: "Upgrade"}}},
	}
	output, warnings := Generate([]Entry{{Name: "Test Item"}}, items)
	if len(warnings) != 0 || len(output) != 1 || output[0].EffectsRemoved[0].Name != "Base" || output[0].EffectsAdded[0].Name != "Upgrade" {
		t.Fatalf("output = %#v, warnings = %#v", output, warnings)
	}
}
