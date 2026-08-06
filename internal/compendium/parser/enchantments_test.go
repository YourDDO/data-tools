package parser

import (
	"reflect"
	"testing"
	"yourddo-data-tools/internal/dataset"
)

func TestProcessEnchText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "recognized dice", input: "{{Dice||1|4}}", want: "1d4"},
		{name: "unknown template", input: "{{Unknown|value}}", want: "{{Unknown|value}}"},
		{name: "unknown then recognized", input: "{{Unknown|value}} then {{Dice||1|4}}", want: "{{Unknown|value}} then 1d4"},
		{name: "recognized then unknown", input: "{{Dice||1|4}} then {{Unknown|value}}", want: "1d4 then {{Unknown|value}}"},
		{name: "nested template exposed by EnchBody", input: "{{EnchBody|Damage: {{Dice||1|4}}}}", want: "Damage: 1d4"},
		{name: "unclosed template", input: "prefix {{Dice||1|4", want: "prefix {{Dice||1|4"},
		{name: "empty text", input: "", want: ""},
		{name: "ordinary text", input: "ordinary text", want: "ordinary text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processEnchText(tt.input); got != tt.want {
				t.Errorf("processEnchText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEnchantments(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		itemType string
		expected []dataset.Enchantment
	}{
		{
			name:     "Legendary Green Steel",
			raw:      "{{LegendaryGreenSteel}}",
			itemType: "Weapon",
			expected: []dataset.Enchantment{
				{
					Name: "Legendary Green Steel",
				},
			},
		},
		{
			name:     "LGSAugments",
			raw:      "{{LGSAugments}}",
			itemType: "Weapon",
			expected: nil,
		},
		{
			name:     "Enhancement Bonus Weapon (Light Hammer)",
			raw:      "{{EnhancementBonus|5}}",
			itemType: "Light Hammer",
			expected: []dataset.Enchantment{
				{Name: "Attack Rolls", Amount: "5", BonusType: "Enhancement"},
				{Name: "Damage Rolls", Amount: "5", BonusType: "Enhancement"},
			},
		},
		{
			name:     "Enhancement Bonus Armor (Docent)",
			raw:      "{{EnhancementBonus|15}}",
			itemType: "Docent",
			expected: []dataset.Enchantment{
				{Name: "Armor Class", Amount: "15", BonusType: "Enhancement"},
			},
		},
		{
			name:     "Enhancement Bonus Shield (Orb)",
			raw:      "{{EnhancementBonus|10}}",
			itemType: "Orb",
			expected: []dataset.Enchantment{
				{Name: "Armor Class", Amount: "10", BonusType: "Enhancement"},
				{Name: "Attack Rolls (Shield)", Amount: "10", BonusType: "Enhancement"},
				{Name: "Damage Rolls (Shield)", Amount: "10", BonusType: "Enhancement"},
			},
		},
		{
			name:     "Enhancement Bonus Shield (Tower Shield)",
			raw:      "{{EnhancementBonus|3}}",
			itemType: "Tower Shield",
			expected: []dataset.Enchantment{
				{Name: "Armor Class", Amount: "3", BonusType: "Enhancement"},
				{Name: "Attack Rolls (Shield)", Amount: "3", BonusType: "Enhancement"},
				{Name: "Damage Rolls (Shield)", Amount: "3", BonusType: "Enhancement"},
			},
		},
		{
			name:     "Enhancement Bonus Buckler",
			raw:      "{{EnhancementBonus|4}}",
			itemType: "Buckler",
			expected: []dataset.Enchantment{
				{Name: "Armor Class", Amount: "4", BonusType: "Enhancement"},
				{Name: "Attack Rolls (Shield)", Amount: "4", BonusType: "Enhancement"},
				{Name: "Damage Rolls (Shield)", Amount: "4", BonusType: "Enhancement"},
			},
		},
		{
			name:     "Enhancement Bonus Generic Weapon",
			raw:      "{{EnhancementBonus|2}}",
			itemType: "Weapon",
			expected: []dataset.Enchantment{
				{Name: "Attack Rolls", Amount: "2", BonusType: "Enhancement"},
				{Name: "Damage Rolls", Amount: "2", BonusType: "Enhancement"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEnchantments(tt.raw, tt.itemType)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseEnchantments() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseEnchantmentsNearlyComplete(t *testing.T) {
	const notes = "This item isn't quite finished, but it's only a step away from completion. Bring it to the forges on the upper floor of Gravenhollow and combine it with melted materials to restore this item to its full potential."

	tests := []struct {
		raw  string
		name string
	}{
		{raw: "{{NearlyComplete|Ability}}", name: "Nearly Complete: Ability Score"},
		{raw: "{{NearlyComplete|HAMP}}", name: "Nearly Complete: Healing Amplification"},
		{raw: "{{NearlyComplete|InsAbility}}", name: "Nearly Complete: Insightful Ability Score"},
		{raw: "{{NearlyComplete|QualityAbility}}", name: "Nearly Complete: Quality Ability Score"},
		{raw: "{{NearlyComplete|Skill}}", name: "Nearly Complete: Skill"},
		{raw: "{{NearlyComplete|SpellFocus}}", name: "Nearly Complete: Spell Focus"},
		{raw: "{{NearlyComplete}}", name: "Nearly Complete"},
		{raw: "{{NearlyComplete|Any}}", name: "Nearly Complete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEnchantments(tt.raw, "Helmet")
			expected := []dataset.Enchantment{{Name: tt.name, Notes: new(notes)}}
			if !reflect.DeepEqual(got, expected) {
				t.Errorf("ParseEnchantments() = %#v, want %#v", got, expected)
			}
		})
	}
}
