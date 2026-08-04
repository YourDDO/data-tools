package parser

import (
	"reflect"
	"testing"
	"yourddo-data-tools/internal/dataset"
)

func TestParseTemplateRaging(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "documented strength bonus",
			raw:  "{{Raging|Strength|3}}",
			want: &dataset.Enchantment{Name: "Raging Strength III", Amount: "3", BonusType: "Rage"},
		},
		{
			name: "negative amount is a penalty",
			raw:  "{{Raging|Dexterity|-2}}",
			want: &dataset.Enchantment{Name: "Raging Dexterity II", Amount: "-2", BonusType: "Penalty"},
		},
		{
			name: "surrounding whitespace is ignored",
			raw:  "  {{Raging| Constitution | 4 }}  ",
			want: &dataset.Enchantment{Name: "Raging Constitution IV", Amount: "4", BonusType: "Rage"},
		},
		{
			name: "missing ability is rejected",
			raw:  "{{Raging||3}}",
			want: nil,
		},
		{
			name: "missing amount is rejected",
			raw:  "{{Raging|Strength|}}",
			want: nil,
		},
		{
			name: "nonnumeric amount is rejected",
			raw:  "{{Raging|Strength|high}}",
			want: nil,
		},
		{
			name: "missing required parameter is rejected",
			raw:  "{{Raging|Strength}}",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateRaging(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTemplateRaging() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseEnchantmentsRaging(t *testing.T) {
	want := []dataset.Enchantment{
		{Name: "Raging Strength III", Amount: "3", BonusType: "Rage"},
	}

	got := ParseEnchantments("{{Raging|Strength|3}}", "Bracers")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}

func TestParseTemplateResistance(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []*dataset.Enchantment
	}{
		{
			name: "documented default resistance",
			raw:  "{{Resistance|3|Insightful}}",
			want: resistanceEnchantments("3", "Insight", ""),
		},
		{
			name: "documented curse resistance",
			raw:  "{{Resistance|4||Curse}}",
			want: resistanceEnchantments("4", "Enhancement", "Curse"),
		},
		{
			name: "enchantment resistance",
			raw:  "{{Resistance|5|Quality|Enchantment}}",
			want: resistanceEnchantments("5", "Quality", "Enchantment"),
		},
		{
			name: "illusion resistance is case insensitive",
			raw:  "{{Resistance|6||illusion}}",
			want: resistanceEnchantments("6", "Resistance", "Illusion"),
		},
		{
			name: "unknown resistance type uses default branch",
			raw:  "{{Resistance|7||Poison}}",
			want: resistanceEnchantments("7", "Resistance", ""),
		},
		{
			name: "missing amount is rejected",
			raw:  "{{Resistance|||Curse}}",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateResistance(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTemplateResistance() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func resistanceEnchantments(amount, bonusType, resistanceType string) []*dataset.Enchantment {
	names := []string{
		"Fortitude Save",
		"Reflex Save",
		"Will Save",
	}
	if resistanceType != "" {
		names = []string{
			"Fortitude Save vs " + resistanceType,
			"Reflex Save vs " + resistanceType,
			"Will Save vs " + resistanceType,
		}
	}

	result := make([]*dataset.Enchantment, 0, len(names))
	for _, name := range names {
		result = append(result, &dataset.Enchantment{
			Name:      name,
			Amount:    amount,
			BonusType: bonusType,
		})
	}
	return result
}
