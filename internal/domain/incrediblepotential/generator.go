// Package incrediblepotential owns the Incredible Potential item selection rule.
package incrediblepotential

import (
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain/itemlist"
)

const Name = "incredible-potential"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Incredible Potential")
	})
}
