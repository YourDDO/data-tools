// Package nearlycomplete owns the Nearly Complete item selection rule.
package nearlycomplete

import (
	"strings"

	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "nearly-complete"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		for _, enchantment := range item.Enchantments {
			if enchantment.Name == "Nearly Complete" || strings.HasPrefix(enchantment.Name, "Nearly Complete: ") {
				return true
			}
		}
		return false
	})
}
