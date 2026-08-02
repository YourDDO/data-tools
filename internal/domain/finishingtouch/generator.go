// Package finishingtouch owns the Finishing Touch item selection rule.
package finishingtouch

import (
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain/itemlist"
)

const Name = "finishing-touch"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Finishing Touch")
	})
}
