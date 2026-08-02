// Package almostthere owns the Almost There item selection rule.
package almostthere

import (
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain/itemlist"
)

const Name = "almost-there"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Almost There")
	})
}
