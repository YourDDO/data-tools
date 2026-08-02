// Package attunedbyheroism owns tiered Heroism attunement selection.
package attunedbyheroism

import (
	"regexp"

	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/itemlist"
)

const Name = "attuned-by-heroism"

var tierName = regexp.MustCompile(`^Attuned (?:by|to) Heroism: Tier [0-9]+$`)

func New() itemlist.Generator {
	return itemlist.New(Name, func(item dataset.ItemData) bool {
		for _, enchantment := range item.Enchantments {
			if tierName.MatchString(enchantment.Name) {
				return true
			}
		}
		return false
	})
}
