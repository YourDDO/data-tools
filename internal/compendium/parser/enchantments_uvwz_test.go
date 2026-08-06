package parser

import (
	"fmt"
	"reflect"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestParseTemplateWeaponEffect(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "default d6 effect",
			raw:  "{{WeaponEffect|Force|9|Impactful}}",
			want: &dataset.Enchantment{Name: "Impactful", Amount: "9d6", Element: "Force", BlastType: "Force"},
		},
		{
			name: "custom die sides and title",
			raw:  "{{WeaponEffect|Sonic|9|Reverberating|Custom Reverberating|8}}",
			want: &dataset.Enchantment{Name: "Custom Reverberating", Amount: "9d8", Element: "Sonic", BlastType: "Sonic"},
		},
		{
			name: "die sides tolerate d prefix",
			raw:  "{{WeaponEffect|Slashing|4|Razor Sharp||d10}}",
			want: &dataset.Enchantment{Name: "Razor Sharp", Amount: "4d10", Element: "Slashing", BlastType: "Slashing"},
		},
		{
			name: "murderous edge fixed effect",
			raw:  "{{WeaponEffect|Murderous Edge}}",
			want: &dataset.Enchantment{
				Name:      "Murderous Edge",
				Amount:    "16d6",
				Element:   "Slashing",
				BlastType: "Murderous Edge",
				Notes:     new("On hit: Slashing damage. This item is considered Metalline, bypassing all kinds of material damage resistance."),
			},
		},
		{
			name: "fixed effect alias",
			raw:  "{{WeaponEffect|BonePaws}}",
			want: &dataset.Enchantment{
				Name:      "Bone Paws",
				BlastType: "BonePaws",
				Notes:     new("While in any Wild Shape, your weapons gain Piercing damage bypass and +1W."),
			},
		},
		{
			name: "fixed effect custom title",
			raw:  "{{WeaponEffect|Flameblade|||Fiery Blade}}",
			want: &dataset.Enchantment{
				Name:      "Fiery Blade",
				BlastType: "Flameblade",
				Notes:     new("This blade is made of pure fire and is surprisingly light to the touch. It innately bypasses the Incorporeal chances of Ethereal monsters."),
			},
		},
		{
			name: "disease alias",
			raw:  "{{WeaponEffect|DiseaseUnholyTear}}",
			want: &dataset.Enchantment{
				Name:      "Disease: Unholy Tear",
				Amount:    "10d6",
				Element:   "Evil",
				BlastType: "DiseaseUnholyTear",
				Notes:     new("Deals Evil damage on each hit to Good enemies and can spread a disease that reduces Armor Class and Positive Healing Amplification."),
			},
		},
		{
			name: "shield spikes dice",
			raw:  "{{WeaponEffect|Shield Spikes|3|||8}}",
			want: &dataset.Enchantment{
				Name:      "Shield Spikes",
				Amount:    "3d8",
				Element:   "Piercing",
				BlastType: "Shield Spikes",
				Notes:     new("Deals extra piercing damage when used to shield bash."),
			},
		},
		{
			name: "tidal burst uses its fixed dice",
			raw:  "{{WeaponEffect|Tidal Burst|9|Bursting Tide}}",
			want: &dataset.Enchantment{
				Name:      "Bursting Tide",
				Amount:    "1d4",
				BlastType: "Tidal Burst",
				Notes:     new("Deals extra damage on each hit. Critical hits also deal 1d8, 2d8, or 3d8 damage for weapons with a x2, x3, or x4 critical multiplier; creatures with the Fire trait take double damage."),
			},
		},
		{
			name: "generic effect requires die count",
			raw:  "{{WeaponEffect|Force}}",
			want: nil,
		},
		{
			name: "shield spikes requires die count",
			raw:  "{{WeaponEffect|Shield Spikes}}",
			want: nil,
		},
		{
			name: "rejects wrong template",
			raw:  "{{DamageEffect|Force|9|Impactful}}",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateWeaponEffect(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateWeaponEffect(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseTemplateWeaponEffectFixedTypes(t *testing.T) {
	tests := []struct {
		raw      string
		wantName string
	}{
		{raw: "{{WeaponEffect|Flameblade}}", wantName: "Flameblade"},
		{raw: "{{WeaponEffect|Frostblade}}", wantName: "Frostblade"},
		{raw: "{{WeaponEffect|Antipodal}}", wantName: "Antipodal"},
		{raw: "{{WeaponEffect|Bone Paws}}", wantName: "Bone Paws"},
		{raw: "{{WeaponEffect|Strength Sapping}}", wantName: "Strength Sapping"},
		{raw: "{{WeaponEffect|Life-Devouring}}", wantName: "Life-Devouring"},
		{raw: "{{WeaponEffect|Disease: Unholy Tear}}", wantName: "Disease: Unholy Tear"},
		{raw: "{{WeaponEffect|Murderous Edge}}", wantName: "Murderous Edge"},
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			got := parseTemplateWeaponEffect(tt.raw)
			if got == nil {
				t.Fatalf("parseTemplateWeaponEffect(%q) returned nil", tt.raw)
			}
			if got.Name != tt.wantName {
				t.Fatalf("parseTemplateWeaponEffect(%q).Name = %q, want %q", tt.raw, got.Name, tt.wantName)
			}
		})
	}
}

func TestParseTemplateWildFrenzy(t *testing.T) {
	const baseNotes = "This weapon has a tendency to drive those it strikes insane. On an attack roll of 20 which is confirmed as a critical hit the target will go wild and attack its own allies for 15 seconds if it fails a Will DC: 25 save. Enemies driven wild in this way, however, have a chance of coming to their senses if damaged."

	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "default",
			raw:  "{{WildFrenzy}}",
			want: &dataset.Enchantment{Name: "Wild Frenzy", Notes: new(baseNotes)},
		},
		{
			name: "explicit basic",
			raw:  "{{WildFrenzy|Basic}}",
			want: &dataset.Enchantment{Name: "Wild Frenzy", Notes: new(baseNotes)},
		},
		{
			name: "custom",
			raw:  "{{WildFrenzy|Custom|50}}",
			want: &dataset.Enchantment{
				Name:  "Wild Frenzy +50",
				Notes: new("This weapon has a tendency to drive those it strikes insane. On an attack roll of 20 which is confirmed as a critical hit the target will go wild and attack its own allies for 15 seconds if it fails a Will DC: 50 save. Enemies driven wild in this way, however, have a chance of coming to their senses if damaged."),
			},
		},
		{
			name: "custom is case insensitive",
			raw:  "{{WildFrenzy| custom | 42 }}",
			want: &dataset.Enchantment{
				Name:  "Wild Frenzy +42",
				Notes: new("This weapon has a tendency to drive those it strikes insane. On an attack roll of 20 which is confirmed as a critical hit the target will go wild and attack its own allies for 15 seconds if it fails a Will DC: 42 save. Enemies driven wild in this way, however, have a chance of coming to their senses if damaged."),
			},
		},
		{
			name: "unknown version uses wiki default",
			raw:  "{{WildFrenzy|Legacy}}",
			want: &dataset.Enchantment{Name: "Wild Frenzy", Notes: new(baseNotes)},
		},
		{
			name: "custom requires DC",
			raw:  "{{WildFrenzy|Custom}}",
			want: nil,
		},
		{
			name: "rejects similarly named template",
			raw:  "{{WildFrenzyOther}}",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateWildFrenzy(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateWildFrenzy(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseEnchantmentsWildFrenzyCustom(t *testing.T) {
	want := []dataset.Enchantment{{
		Name:  "Wild Frenzy +50",
		Notes: new("This weapon has a tendency to drive those it strikes insane. On an attack roll of 20 which is confirmed as a critical hit the target will go wild and attack its own allies for 15 seconds if it fails a Will DC: 50 save. Enemies driven wild in this way, however, have a chance of coming to their senses if damaged."),
	}}

	got := ParseEnchantments("{{WildFrenzy|Custom|50}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}

func TestParseTemplateWounding(t *testing.T) {
	const onHitNotes = "A wounding weapon deals %d %s of Constitution damage from blood loss when it hits a creature. Creatures immune to critical hits are immune to the Constitution damage dealt by this weapon."

	tests := []struct {
		name string
		raw  string
		want *dataset.Enchantment
	}{
		{
			name: "basic",
			raw:  "{{Wounding}}",
			want: &dataset.Enchantment{Name: "Wounding", Notes: new("Wounding: " + fmt.Sprintf(onHitNotes, 1, "point"))},
		},
		{
			name: "greater",
			raw:  "{{Wounding|Greater}}",
			want: &dataset.Enchantment{Name: "Greater Wounding", Notes: new("Greater Wounding: " + fmt.Sprintf(onHitNotes, 3, "points"))},
		},
		{
			name: "specific",
			raw:  "{{Wounding|Specific|4}}",
			want: &dataset.Enchantment{Name: "Wounding 4", Notes: new("Wounding 4: " + fmt.Sprintf(onHitNotes, 4, "points"))},
		},
		{
			name: "critical",
			raw:  "{{Wounding|Critical|4}}",
			want: &dataset.Enchantment{Name: "Critical Wounding 4", Notes: new("Critical Wounding 4: This deadly weapon saps the health from your enemies, dealing 4 Constitution damage on each critical hit.")},
		},
		{
			name: "custom title",
			raw:  "{{Wounding|Critical|4|Bloodletter}}",
			want: &dataset.Enchantment{Name: "Bloodletter", Notes: new("Bloodletter: This deadly weapon saps the health from your enemies, dealing 4 Constitution damage on each critical hit.")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateWounding(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateWounding(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
