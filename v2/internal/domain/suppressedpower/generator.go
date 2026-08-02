// Package suppressedpower owns the Suppressed Power item selection rule.
package suppressedpower

import (
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "suppressed-power"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Suppressed Power")
	})
}
