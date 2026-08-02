// Package lostpurpose owns the Lost Purpose item selection rule.
package lostpurpose

import (
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "lost-purpose"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Lost Purpose")
	})
}
