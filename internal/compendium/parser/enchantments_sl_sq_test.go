package parser

import (
	"fmt"
	"reflect"
	"testing"
	"yourddo-data-tools/internal/dataset"
)

func TestParseTemplateSpellFocus(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []*dataset.Enchantment
	}{
		{
			name: "Single School",
			raw:  "{{SpellFocus|Necromancy|2|Insight}}",
			expected: []*dataset.Enchantment{
				{Name: "Spell DC: Necromancy", Amount: "2", BonusType: "Insight"},
			},
		},
		{
			name: "Mastery",
			raw:  "{{SpellFocus|Mastery|3}}",
			expected: func() []*dataset.Enchantment {
				out := make([]*dataset.Enchantment, 0, len(spellSchools))
				for _, school := range spellSchools {
					out = append(out, &dataset.Enchantment{
						Name:      fmt.Sprintf("Spell DC: %s", school),
						Amount:    "3",
						BonusType: "Equipment",
					})
				}
				return out
			}(),
		},
		{
			name: "Rune Arm",
			raw:  "{{SpellFocus|Rune Arm|1|Artifact}}",
			expected: []*dataset.Enchantment{
				{Name: "Rune Arm: DC", Amount: "1", BonusType: "Artifact"},
			},
		},
		{
			name: "Equipoised",
			raw:  "{{SpellFocus|Equipoised}}",
			expected: func() []*dataset.Enchantment {
				out := make([]*dataset.Enchantment, 0, len(spellSchools))
				for _, school := range spellSchools {
					out = append(out, &dataset.Enchantment{
						Name:      fmt.Sprintf("Spell DC: %s", school),
						Amount:    "2",
						BonusType: "Exceptional",
					})
				}
				return out
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateSpellFocus(tt.raw)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseTemplateSpellFocus() = %#v, want %#v", got, tt.expected)
			}
		})
	}
}

func TestParseEnchantmentsSpellIntensity(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []dataset.Enchantment
	}{
		{
			name: "physical spell powers use the default bonus",
			raw:  "{{SpellIntensity|Kinetic|15}}",
			want: []dataset.Enchantment{
				{Name: "Spell Critical Damage: Force", Amount: "15", BonusType: "Equipment", Element: "Force"},
				{Name: "Spell Critical Damage: Piercing", Amount: "15", BonusType: "Equipment", Element: "Piercing"},
				{Name: "Spell Critical Damage: Slashing", Amount: "15", BonusType: "Equipment", Element: "Slashing"},
				{Name: "Spell Critical Damage: Bludgeoning", Amount: "15", BonusType: "Equipment", Element: "Bludgeoning"},
			},
		},
		{
			name: "compound group emits every spell power",
			raw:  "{{SpellIntensity|Silver Flame|30|Quality}}",
			want: []dataset.Enchantment{
				{Name: "Spell Critical Damage: Positive Energy", Amount: "30", BonusType: "Quality", Element: "Positive Energy"},
				{Name: "Spell Critical Damage: Light", Amount: "30", BonusType: "Quality", Element: "Light"},
				{Name: "Spell Critical Damage: Chaos", Amount: "30", BonusType: "Quality", Element: "Chaos"},
				{Name: "Spell Critical Damage: Evil", Amount: "30", BonusType: "Quality", Element: "Evil"},
				{Name: "Spell Critical Damage: Good", Amount: "30", BonusType: "Quality", Element: "Good"},
				{Name: "Spell Critical Damage: Lawful", Amount: "30", BonusType: "Quality", Element: "Lawful"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEnchantments(tt.raw, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseEnchantments() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseEnchantmentsTheMummysGift(t *testing.T) {
	want := []dataset.Enchantment{{
		Name:  "The Mummy's Gift",
		Notes: new("Being struck in melee has a small chance to return some lost Hitpoints and Spellpoints to you. Offensive spells have a chance to grant 100 temporary spellpoints. This has a one minute cooldown."),
	}}

	got := ParseEnchantments("{{TheMummysGift}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}
