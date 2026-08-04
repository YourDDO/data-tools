package parser

import (
	"reflect"
	"testing"
	"yourddo-data-tools/internal/dataset"
)

func TestParseFiligreeSets(t *testing.T) {
	wikitext := `
=== [[Mists of Ravenloft]] Filigree sets ===
{| class="compendiumtable compendiumtablenonsortable fullwidth"
! style="width: 25%;"| Set name || style="width: 100%;"| Description 
|-
|{{FiligreeTitle|The Beast's Mantle}}
|<section begin=the beast's mantle />
: 2 Pieces Equipped: +10 Natural Armor
: 3 Pieces Equipped: +10% {{FiligreeItemEnchantment|Armor Piercing||Fortification Bypass}}
: 4 Pieces Equipped: +2 {{ColorText|Imbue|Imbue}} Dice<section end=the beast's mantle />
|-
|{{FiligreeTitle|Celerity}}
|<section begin=celerity />
: 2 Pieces Equipped: +2 Doublestrike and Doubleshot
: 3 Pieces Equipped: Permanent Haste spell<section end=celerity />
|-
|{{FiligreeTitle|Electrocution}}
|<section begin=electrocution />
: 2 Pieces Equipped: +50 Electric Spellpower 
: 3 Pieces Equipped: +2 {{ColorText|Imbue|Imbue}} Dice
: 4 Pieces Equipped: You gain a permanent Energy Sheath (Electric)<section end=electrocution />
|-
|{{FiligreeTitle|Eye of the Beholder}}
|<section begin=eye of the beholder />
: 2 Pieces Equipped: +1 Transmutation and Conjuration DCs
: 3 Pieces Equipped: +1 Spell DC
: 4 Pieces Equipped: +2 to the DCs of all spells
: 5 Pieces Equipped: Your Melee and Ranged attacks have a chance to reduce enemy PRR and MRR by 10 for 10 seconds.<section end=eye of the beholder />
|}
`

	expected := []dataset.FiligreeSet{
		{
			Name: "The Beast's Mantle",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Natural Armor", Modifier: 10.0},
					},
				},
				{
					Threshold: 3,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Fortification Bypass", Modifier: "10%"},
					},
				},
				{
					Threshold: 4,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Imbue", Modifier: 2.0},
					},
				},
			},
		},
		{
			Name: "Celerity",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Doublestrike", Modifier: 2.0},
						{Name: "Doubleshot", Modifier: 2.0},
					},
				},
				{
					Threshold: 3,
					Enhancements: []dataset.PartialEnhancementOut{
						{Description: "Permanent Haste spell"},
					},
				},
			},
		},
		{
			Name: "Electrocution",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Spell Power: Electric", Modifier: 50.0},
					},
				},
				{
					Threshold: 3,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Imbue", Modifier: 2.0},
					},
				},
				{
					Threshold: 4,
					Enhancements: []dataset.PartialEnhancementOut{
						{Notes: "You gain a permanent Energy Sheath (Electric)"},
					},
				},
			},
		},
		{
			Name: "Eye of the Beholder",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Spell DC: Transmutation", Modifier: 1.0},
						{Name: "Spell DC: Conjuration", Modifier: 1.0},
					},
				},
				{
					Threshold: 3,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Spell DC: Abjuration", Modifier: 1.0},
						{Name: "Spell DC: Breath Weapon", Modifier: 1.0},
						{Name: "Spell DC: Conjuration", Modifier: 1.0},
						{Name: "Spell DC: Divination", Modifier: 1.0},
						{Name: "Spell DC: Enchantment", Modifier: 1.0},
						{Name: "Spell DC: Evocation", Modifier: 1.0},
						{Name: "Spell DC: Illusion", Modifier: 1.0},
						{Name: "Spell DC: Necromancy", Modifier: 1.0},
						{Name: "Spell DC: Transmutation", Modifier: 1.0},
					},
				},
				{
					Threshold: 4,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Spell DC: Abjuration", Modifier: 2.0},
						{Name: "Spell DC: Breath Weapon", Modifier: 2.0},
						{Name: "Spell DC: Conjuration", Modifier: 2.0},
						{Name: "Spell DC: Divination", Modifier: 2.0},
						{Name: "Spell DC: Enchantment", Modifier: 2.0},
						{Name: "Spell DC: Evocation", Modifier: 2.0},
						{Name: "Spell DC: Illusion", Modifier: 2.0},
						{Name: "Spell DC: Necromancy", Modifier: 2.0},
						{Name: "Spell DC: Transmutation", Modifier: 2.0},
					},
				},
				{
					Threshold: 5,
					Enhancements: []dataset.PartialEnhancementOut{
						{Notes: "Your Melee and Ranged attacks have a chance to reduce enemy PRR and MRR by 10 for 10 seconds."},
					},
				},
			},
		},
	}

	result := ParseFiligreeSets(wikitext)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Result mismatch.\nGot: %+v\nExpected: %+v", result, expected)
	}
}

func TestParseFiligreeSetsNormalization(t *testing.T) {
	wikitext := `
{| class="compendiumtable"
|-
|{{FiligreeTitle|New Mapping Set}}
|
: 2 Pieces Equipped: +2 uses of Wild Empathy
: 3 Pieces Equipped: +20 Universal Spell Power
: 4 Pieces Equipped: You gain True Seeing
: 5 Pieces Equipped: +3 Lay on Hands Charges
|-
|{{FiligreeTitle|Another Set}}
|
: 2 Pieces Equipped: Displacement. Stacks with other sources.
: 3 Pieces Equipped: +2 to the DC of Assassinate
: 4 Pieces Equipped: +5 Open Locks, Disable Device, Spot, Search
: 5 Pieces Equipped: You are immune to Petrification
|-
|{{FiligreeTitle|To Hell and Back}}
|
: 4 Pieces Equipped: +2 to all Saving Throws
|-
|{{FiligreeTitle|Treachery}}
|
: 2 Pieces Equipped: +5 Melee and Ranged Power
|-
|{{FiligreeTitle|Touch of Grace}}
|
: 2 Pieces Equipped: +50 Spell Points
|}
`
	expected := []dataset.FiligreeSet{
		{
			Name: "New Mapping Set",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Wild Empathy: Max Charges", Modifier: 2.0},
					},
				},
				{
					Threshold: 3,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Spell Power: Universal", Modifier: 20.0},
					},
				},
				{
					Threshold: 4,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "True Seeing"},
					},
				},
				{
					Threshold: 5,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Lay on Hands: Max Charges", Modifier: 3.0},
					},
				},
			},
		},
		{
			Name: "Another Set",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Displacement"},
					},
				},
				{
					Threshold: 3,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Assassinate DCs", Modifier: 2.0},
					},
				},
				{
					Threshold: 4,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Skill: Open Lock", Modifier: 5.0},
						{Name: "Skill: Disable Device", Modifier: 5.0},
						{Name: "Skill: Spot", Modifier: 5.0},
						{Name: "Skill: Search", Modifier: 5.0},
					},
				},
				{
					Threshold: 5,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Immunity: Petrification"},
					},
				},
			},
		},
		{
			Name: "To Hell and Back",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 4,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Fortitude Save", Modifier: 2.0},
						{Name: "Reflex Save", Modifier: 2.0},
						{Name: "Will Save", Modifier: 2.0},
					},
				},
			},
		},
		{
			Name: "Treachery",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Melee Power", Modifier: 5.0},
						{Name: "Ranged Power", Modifier: 5.0},
					},
				},
			},
		},
		{
			Name: "Touch of Grace",
			Bonuses: []dataset.FiligreeSetBonus{
				{
					Threshold: 2,
					Enhancements: []dataset.PartialEnhancementOut{
						{Name: "Spell Points", Modifier: 50.0},
					},
				},
			},
		},
	}

	result := ParseFiligreeSets(wikitext)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Normalization Result mismatch.\nGot: %+v\nExpected: %+v", result, expected)
	}
}
