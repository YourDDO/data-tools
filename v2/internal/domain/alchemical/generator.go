// Package alchemical owns the Alchemical prototype item selection rule.
package alchemical

import (
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "alchemical"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Alchemical (Prototype)")
	})
}
