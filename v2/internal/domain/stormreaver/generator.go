// Package stormreaver owns Stormreaver Monument upgrade pairing.
package stormreaver

import (
	"context"

	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain"
	"yourddo-data-tools/v2/internal/domain/itemlist"
	"yourddo-data-tools/v2/internal/domain/upgrade"
)

const Name = "stormreaver-monument"

type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

func (Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	return upgrade.Generate(ctx, Name, func(item dataset.ItemData) bool {
		return itemlist.HasEnchantment(item, "Upgradeable Item (Stormreaver)")
	}, master, outputRoot)
}
