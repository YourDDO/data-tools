// Package catalystcrafting owns the broad Madness catalyst selection rule.
package catalystcrafting

import (
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "catalyst-crafting"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Trace of Madness") ||
			itemlist.HasEnchantment(item, "Suppressed Power") ||
			itemlist.HasEnchantment(item, "Lost Purpose")
	})
}
