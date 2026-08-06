package parser

import (
	"reflect"
	"testing"
	"yourddo-data-tools/internal/dataset"
)

func TestParseTemplateIncite(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []*dataset.Enchantment
	}{
		{
			name: "default melee attack",
			raw:  "{{Incite|10}}",
			want: []*dataset.Enchantment{
				{Name: "Melee Threat", Amount: "0.1"},
			},
		},
		{
			name: "spell attack ignores presentation title",
			raw:  "{{Incite|-10|spell||Anathema}}",
			want: []*dataset.Enchantment{
				{Name: "Spell Threat", Amount: "-0.1"},
			},
		},
		{
			name: "spell and ranged attacks retain distinct semantic identities",
			raw:  "{{Incite|-15|spellranged||Stealth Strike}}",
			want: []*dataset.Enchantment{
				{Name: "Ranged Threat", Amount: "-0.15"},
				{Name: "Spell Threat", Amount: "-0.15"},
			},
		},
		{
			name: "spell attack without custom title",
			raw:  "{{Incite|-15|spell}}",
			want: []*dataset.Enchantment{
				{Name: "Spell Threat", Amount: "-0.15"},
			},
		},
		{
			name: "bonus type is the third parameter",
			raw:  "{{Incite|9|melee|Quality}}",
			want: []*dataset.Enchantment{
				{Name: "Melee Threat", Amount: "0.09", BonusType: "Quality"},
			},
		},
		{
			name: "spell and melee attacks",
			raw:  "{{Incite|20|SpellMelee|Insight}}",
			want: []*dataset.Enchantment{
				{Name: "Melee Threat", Amount: "0.2", BonusType: "Insight"},
				{Name: "Spell Threat", Amount: "0.2", BonusType: "Insight"},
			},
		},
		{
			name: "all attack types",
			raw:  "{{Incite|25|All|Artifact}}",
			want: []*dataset.Enchantment{
				{Name: "Melee Threat", Amount: "0.25", BonusType: "Artifact"},
				{Name: "Ranged Threat", Amount: "0.25", BonusType: "Artifact"},
				{Name: "Spell Threat", Amount: "0.25", BonusType: "Artifact"},
			},
		},
		{
			name: "explicit percent suffix is not scaled twice",
			raw:  "{{Incite|-20%|All}}",
			want: []*dataset.Enchantment{
				{Name: "Melee Threat", Amount: "-0.2"},
				{Name: "Ranged Threat", Amount: "-0.2"},
				{Name: "Spell Threat", Amount: "-0.2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateIncite(tt.raw, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTemplateIncite() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
