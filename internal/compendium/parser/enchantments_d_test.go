package parser

import (
	"reflect"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestParseTemplateDisintegration(t *testing.T) {
	defaultNotes := "This weapon has a dark, insidious power deep within. Occasionally, this power lashes out violently at enemies and attempts to disintegrate them."
	utterNotes := defaultNotes + " This disintegrate is incredibly powerful, and will utterly destroy weaker foes."

	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "default",
			raw:  "{{Disintegration}}",
			want: &dataset.Enchantment{Name: "Disintegration", Notes: new(defaultNotes)},
		},
		{
			name: "utter",
			raw:  "{{Disintegration|Utter}}",
			want: &dataset.Enchantment{Name: "Utter Disintegration", Notes: new(utterNotes)},
		},
		{
			name: "damage",
			raw:  "{{Disintegration|damage|7}}",
			want: &dataset.Enchantment{
				Name:  "Disintigrate +7",
				Notes: new("Strikes with this weapon have a small chance to call forth a spike of entropic force, dealing 7d20+72 untyped damage."),
			},
		},
		{
			name: "switch is case insensitive",
			raw:  "{{Disintegration|DaMaGe| 4 }}",
			want: &dataset.Enchantment{
				Name:  "Disintigrate +4",
				Notes: new("Strikes with this weapon have a small chance to call forth a spike of entropic force, dealing 4d20+45 untyped damage."),
			},
		},
		{
			name: "damage requires a numeric amount",
			raw:  "{{Disintegration|damage|unknown}}",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateDisintegration(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateDisintegration(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseEnchantmentsDisintegrationDamage(t *testing.T) {
	want := []dataset.Enchantment{{
		Name:  "Disintigrate +7",
		Notes: new("Strikes with this weapon have a small chance to call forth a spike of entropic force, dealing 7d20+72 untyped damage."),
	}}

	got := ParseEnchantments("{{Disintegration|damage|7}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}

func TestParseTemplateDiversion(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []*dataset.Enchantment
	}{
		{
			name: "melee reduction is a negative decimal",
			raw:  "{{Diversion|28|Melee}}",
			want: []*dataset.Enchantment{{
				Name: "Melee Threat", Amount: "-0.28", BonusType: "Enhancement",
			}},
		},
		{
			name: "spell reduction preserves decimal precision and bonus type",
			raw:  "{{Diversion|6.5|Spell|Quality}}",
			want: []*dataset.Enchantment{{
				Name: "Spell Threat", Amount: "-0.065", BonusType: "Quality",
			}},
		},
		{
			name: "combined styles emit canonical threat names",
			raw:  "{{Diversion|10|MeleeRange|Insight|Custom Diversion}}",
			want: []*dataset.Enchantment{
				{Name: "Melee Threat", Amount: "-0.1", BonusType: "Insight"},
				{Name: "Ranged Threat", Amount: "-0.1", BonusType: "Insight"},
			},
		},
		{
			name: "a signed input remains a reduction",
			raw:  "{{Diversion|-15|All}}",
			want: []*dataset.Enchantment{
				{Name: "Melee Threat", Amount: "-0.15", BonusType: "Enhancement"},
				{Name: "Ranged Threat", Amount: "-0.15", BonusType: "Enhancement"},
				{Name: "Spell Threat", Amount: "-0.15", BonusType: "Enhancement"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateDiversion(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateDiversion(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
