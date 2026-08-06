package parser

import (
	"reflect"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestParseTemplateAntimagicSpike(t *testing.T) {
	defaultNotes := "This item was made from powerful antimagic materials. When scoring a critical hit on an enemy with your ranged or melee weapons, the target must make a Fortitude DC: 28 save or be unable to cast spells for a brief duration."
	customNotes := "This item was made from powerful antimagic materials. When scoring a critical hit on an enemy, it must make a Fortitude DC: 45 save or be unable to cast spells for a brief duration."

	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "default",
			raw:  "{{AntimagicSpike}}",
			want: &dataset.Enchantment{Name: "Antimagic Spike", Notes: new(defaultNotes)},
		},
		{
			name: "custom DC",
			raw:  "{{AntimagicSpike|custom|45}}",
			want: &dataset.Enchantment{Name: "Antimagic Spike +45", Notes: new(customNotes)},
		},
		{
			name: "switch is case insensitive",
			raw:  "{{AntimagicSpike|Custom| 45 }}",
			want: &dataset.Enchantment{Name: "Antimagic Spike +45", Notes: new(customNotes)},
		},
		{
			name: "other type uses default effect",
			raw:  "{{AntimagicSpike|regular|45}}",
			want: &dataset.Enchantment{Name: "Antimagic Spike", Notes: new(defaultNotes)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateAntimagicSpike(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateAntimagicSpike(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseTemplateAC(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "standard source defaults to enhancement",
			raw:  "{{AC|Deflection|5}}",
			want: &dataset.Enchantment{Name: "Armor Class (Deflection)", Amount: "5", BonusType: "Enhancement"},
		},
		{
			name: "armor percent",
			raw:  "{{AC|armorpercent|25|Quality}}",
			want: &dataset.Enchantment{Name: "Armor Class (Armor)", Amount: "25%", BonusType: "Quality"},
		},
		{
			name: "rough hide defaults to primal",
			raw:  "{{AC|Rough Hide|3}}",
			want: &dataset.Enchantment{Name: "Armor Class (Rough Hide)", Amount: "3", BonusType: "Primal"},
		},
		{
			name: "heightened awareness defaults to insight",
			raw:  "{{AC|Heightened Awareness|4}}",
			want: &dataset.Enchantment{Name: "Armor Class (Heightened Awareness)", Amount: "4", BonusType: "Insight"},
		},
		{
			name: "protection from evil defaults to deflection",
			raw:  "{{AC|Protection From Evil|2}}",
			want: &dataset.Enchantment{Name: "Armor Class (Protection From Evil)", Amount: "2", BonusType: "Deflection", Notes: new("vs. Evil")},
		},
		{
			name: "armor class has no bonus type",
			raw:  "{{AC|Armor Class|1}}",
			want: &dataset.Enchantment{Name: "Armor Class (Base)", Amount: "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateAC(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateAC(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseEnchantmentsAntimagicSpikeCustom(t *testing.T) {
	want := []dataset.Enchantment{{
		Name:  "Antimagic Spike +45",
		Notes: new("This item was made from powerful antimagic materials. When scoring a critical hit on an enemy, it must make a Fortitude DC: 45 save or be unable to cast spells for a brief duration."),
	}}

	got := ParseEnchantments("{{AntimagicSpike|custom|45}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}
