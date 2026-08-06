package parser

import (
	"reflect"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestParseTemplateMagmaSurge(t *testing.T) {
	defaultNotes := "This weapon stores the immeasurable heat of the planet's molten mantle. When this weapon is used, superheated magma occasionally surges to the surface, slowing an enemy down and inflicting massive fire damage over time."

	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "default",
			raw:  "{{MagmaSurge}}",
			want: &dataset.Enchantment{Name: "Magma Surge", Notes: new(defaultNotes)},
		},
		{
			name: "legendary",
			raw:  "{{MagmaSurge|Legendary}}",
			want: &dataset.Enchantment{Name: "Legendary Magma Surge", Notes: new(defaultNotes)},
		},
		{
			name: "damage",
			raw:  "{{MagmaSurge|damage|10}}",
			want: &dataset.Enchantment{
				Name:  "Magma Surge +10",
				Notes: new("Strikes with this weapon have a small chance to call forth a surge of superheated magma, dealing 10d20+99 fire damage."),
			},
		},
		{
			name: "switch is case insensitive",
			raw:  "{{MagmaSurge|DaMaGe| 4 }}",
			want: &dataset.Enchantment{
				Name:  "Magma Surge +4",
				Notes: new("Strikes with this weapon have a small chance to call forth a surge of superheated magma, dealing 4d20+45 fire damage."),
			},
		},
		{
			name: "damage requires a numeric amount",
			raw:  "{{MagmaSurge|damage|unknown}}",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateMagmaSurge(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateMagmaSurge(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseEnchantmentsMagmaSurgeDamage(t *testing.T) {
	want := []dataset.Enchantment{{
		Name:  "Magma Surge +10",
		Notes: new("Strikes with this weapon have a small chance to call forth a surge of superheated magma, dealing 10d20+99 fire damage."),
	}}

	got := ParseEnchantments("{{MagmaSurge|damage|10}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}

func TestParseEnchantmentsMemoryOfAnimatedObjects(t *testing.T) {
	want := []dataset.Enchantment{
		{Name: "Spell Power: Repair", Amount: "171", BonusType: "Equipment"},
		{Name: "Spell Power: Rust", Amount: "171", BonusType: "Equipment"},
		{Name: "Spell Critical Chance: Repair", Amount: "24", BonusType: "Equipment"},
		{Name: "Spell Critical Chance: Rust", Amount: "24", BonusType: "Equipment"},
		{Name: "Spell Critical Damage: Repair", Amount: "25", BonusType: "Enhancement", Element: "Repair"},
		{Name: "Spell Critical Damage: Rust", Amount: "25", BonusType: "Enhancement", Element: "Rust"},
	}

	got := ParseEnchantments("{{MemoryOfAnimatedObjects}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}

func TestParseTemplateMaiming(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "normal default",
			raw:  "{{Maiming}}",
			want: &dataset.Enchantment{Name: "Maiming", Element: "Untyped", Notes: new("On critical hit: x2 1d6, x3 2d6, or x4 3d6 untyped damage.")},
		},
		{
			name: "greater",
			raw:  "{{Maiming|Greater}}",
			want: &dataset.Enchantment{Name: "Greater Maiming", Element: "Untyped", Notes: new("On critical hit: x2 4d6, x3 12d6, or x4 16d6 untyped damage.")},
		},
		{
			name: "augment",
			raw:  "{{Maiming|augment}}",
			want: &dataset.Enchantment{Name: "Greater Maiming", Element: "Untyped", Notes: new("On critical hit: x2 8d6, x3 12d6, or x4 16d6 untyped damage.")},
		},
		{
			name: "new style",
			raw:  "{{Maiming|New|9}}",
			want: &dataset.Enchantment{Name: "Maiming 9", Amount: "9d8", Element: "Untyped", Notes: new("On critical hit: untyped damage.")},
		},
		{
			name: "weapon effect",
			raw:  "{{Maiming|Weapon|9}}",
			want: &dataset.Enchantment{Name: "Weapon's Maiming Effect +9", Notes: new("Does additional damage on critical hits.")},
		},
		{
			name: "new style requires an amount",
			raw:  "{{Maiming|New}}",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateMaiming(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateMaiming(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
