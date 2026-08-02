// Package finishingtouch owns the Finishing Touch item selection rule.
package finishingtouch

import (
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "finishing-touch"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Finishing Touch")
	})
}
