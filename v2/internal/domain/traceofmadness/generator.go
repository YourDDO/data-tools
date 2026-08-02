// Package traceofmadness owns the Trace of Madness item selection rule.
package traceofmadness

import (
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "trace-of-madness"

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Trace of Madness")
	})
}
