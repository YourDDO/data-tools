// Package nearlyfinished owns the Nearly Finished item selection rule.
package nearlyfinished

import (
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "nearly-finished"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Nearly Finished")
	})
}
