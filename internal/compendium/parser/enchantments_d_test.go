package parser

import (
	"reflect"
	"strconv"
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

func TestParseTemplateDragonmark(t *testing.T) {
	lesserNotes := func(increase int) *string {
		return new("This will increase the total number of Lesser Dragonmarks you can use by " + strconv.Itoa(increase) + ". However, these additional Lesser Dragonmarks will only take effect after the wielder rests.")
	}
	greaterNotes := func(increase int) *string {
		return new("This will increase the total number of Greater Dragonmarks you can use by " + strconv.Itoa(increase) + ". However, these additional Greater Dragonmarks will only take effect after the wielder rests.")
	}
	greaterChimeraNotes := "This weapon's ultimate form may only be realized by those who bear a Dragonmark.\n" +
		"* With any Dragonmark, add:\n" +
		"** Feat: Exotic Weapon Proficiency: Bastard Sword\n" +
		"** +7 Enhancement Bonus\n" +
		"* With 2 Dragonmarks, add:\n" +
		"** Incite +10%\n" +
		"** Parrying IV\n" +
		"** +8 Enhancement Bonus\n" +
		"* With 3 Dragonmarks, add:\n" +
		"** Incite +15%\n" +
		"** Parrying VIII\n" +
		"** +7 Enhancement Bonus\n" +
		"* With Greater Dragonmark of Sentinel, add\n" +
		"** Incite +20%\n" +
		"** Insightful Fortification +50%\n" +
		"** +10 Enhancement Bonus\n" +
		"* With Greater Dragonmark of Storm, add\n" +
		"** Electricity Absorption +38%\n" +
		"** Whirlwind Ward\n" +
		"* With Greater Dragonmark of Healing, add\n" +
		"** Healing Amplification +38\n" +
		"* With Greater Dragonmark of Making, add\n" +
		"** Repair Amplification +38\n" +
		"* With Greater Dragonmark of Warding, add\n" +
		"** Spell Resistance +38"
	legendaryChimeraNotes := "This weapon's ultimate form may only be realized by those who bear a Dragonmark.\n" +
		"* With any Dragonmark, add:\n" +
		"** Feat: Exotic Weapon Proficiency: Bastard Sword\n" +
		"* With Greater Dragonmark of Sentinel, add\n" +
		"** Incite +124%\n" +
		"* With Greater Dragonmark of Shadow, add\n" +
		"** Lesser Displacement\n" +
		"* With Greater Dragonmark of Scribing, add\n" +
		"** Armor Class +13\n" +
		"* With Greater Dragonmark of Passage, add\n" +
		"** Resistance +11\n" +
		"* With Greater Dragonmark of Finding, add\n" +
		"** Insightful Fortification +73%"

	tests := []struct {
		name string
		raw  string
		want []*dataset.Enchantment
	}{
		{
			name: "lesser defaults to three uses without an increase type",
			raw:  "{{Dragonmark|Lesser}}",
			want: []*dataset.Enchantment{{
				Name: "Lesser Dragonmark Enhancement", Amount: "3", BonusType: "Enhancement", Notes: lesserNotes(3),
			}},
		},
		{
			name: "lesser normal increase type uses the default route",
			raw:  "{{Dragonmark|Lesser|Normal}}",
			want: []*dataset.Enchantment{{
				Name: "Normal Lesser Dragonmark Enhancement", Amount: "3", BonusType: "Enhancement", Notes: lesserNotes(3),
			}},
		},
		{
			name: "lesser minor increase type",
			raw:  "{{Dragonmark|Lesser|Minor}}",
			want: []*dataset.Enchantment{{
				Name: "Minor Lesser Dragonmark Enhancement", Amount: "1", BonusType: "Enhancement", Notes: lesserNotes(1),
			}},
		},
		{
			name: "lesser major increase type",
			raw:  "{{Dragonmark|Lesser|Major}}",
			want: []*dataset.Enchantment{{
				Name: "Major Lesser Dragonmark Enhancement", Amount: "5", BonusType: "Enhancement", Notes: lesserNotes(5),
			}},
		},
		{
			name: "greater defaults to three uses without an increase type",
			raw:  "{{Dragonmark|Greater}}",
			want: []*dataset.Enchantment{{
				Name: "Greater Dragonmark Enhancement", Amount: "3", BonusType: "Enhancement", Notes: greaterNotes(3),
			}},
		},
		{
			name: "greater normal increase type uses the default route",
			raw:  "{{Dragonmark|Greater|Normal}}",
			want: []*dataset.Enchantment{{
				Name: "Normal Greater Dragonmark Enhancement", Amount: "3", BonusType: "Enhancement", Notes: greaterNotes(3),
			}},
		},
		{
			name: "greater minor increase type is case insensitive",
			raw:  "{{Dragonmark| GrEaTeR | MiNoR }}",
			want: []*dataset.Enchantment{{
				Name: "Minor Greater Dragonmark Enhancement", Amount: "1", BonusType: "Enhancement", Notes: greaterNotes(1),
			}},
		},
		{
			name: "greater major increase type",
			raw:  "{{Dragonmark|Greater|Major}}",
			want: []*dataset.Enchantment{{
				Name: "Major Greater Dragonmark Enhancement", Amount: "5", BonusType: "Enhancement", Notes: greaterNotes(5),
			}},
		},
		{
			name: "chimeras vitality",
			raw:  "{{Dragonmark|Chimera's Vitality}}",
			want: []*dataset.Enchantment{{
				Name: "Chimera's Vitality",
				Notes: new("This item grants extra bonuses if the wearer has been bestowed with Dragonmarks. You gain +1 use of Greater Dragonmark and +5 Quality PRR per Dragonmark you have.\n\n" +
					"Dragonmark of Sentinel bonus: You gain the effects of the Deific Warding feat. This does not stack with the feat itself."),
			}},
		},
		{
			name: "historic chimeras vitality",
			raw:  "{{Dragonmark|Historic Chimera's Vitality}}",
			want: []*dataset.Enchantment{{
				Name:  "Chimera's Vitality",
				Notes: new("This item grants extra bonuses if the wearer has been bestowed with Dragonmarks. 1 Dragonmark feat: the item will grant 10 hitpoints. 2 Dragonmark feats: the item will grant 15 hitpoints. 3 Dragonmark feats: the item will grant 20 hitpoints and Spell Resistance 30. The hitpoint bonuses will stack with all other hitpoint-granting effects except for themselves."),
			}},
		},
		{
			name: "greater chimeras ferocity",
			raw:  "{{Dragonmark|Greater Chimera's Ferocity}}",
			want: []*dataset.Enchantment{{Name: "Greater Chimera's Ferocity", Notes: new(greaterChimeraNotes)}},
		},
		{
			name: "legendary chimeras ferocity",
			raw:  "{{Dragonmark|Legendary Chimera's Ferocity}}",
			want: []*dataset.Enchantment{{Name: "Legendary Chimera's Ferocity", Notes: new(legendaryChimeraNotes)}},
		},
		{
			name: "legendary compatibility alias from the template documentation",
			raw:  "{{Dragonmark|Legendary}}",
			want: []*dataset.Enchantment{{Name: "Legendary Chimera's Ferocity", Notes: new(legendaryChimeraNotes)}},
		},
		{
			name: "historic chimeras ferocity",
			raw:  "{{Dragonmark|Historic Chimera's Ferocity}}",
			want: []*dataset.Enchantment{{
				Name:  "Chimera's Ferocity",
				Notes: new("This effect grants extra bonuses if the wearer has been bestowed with Dragonmarks: 1 or more Dragonmarks: item becomes +7 and grants Bastard Sword Proficiency. 2 or more Dragonmarks: item becomes +8 and grants Incite +10% and Greater Parrying. 3 Dragonmarks: item grants Incite +15% and Superior Parrying. If the wearer has the Greater Dragonmark of Sentinel, it will also become +10 and grants Incite +20% and Fortified Defenses +50%."),
			}},
		},
		{
			name: "unknown variant emits no enchantment",
			raw:  "{{Dragonmark|Unknown}}",
		},
		{
			name: "missing variant emits no enchantment",
			raw:  "{{Dragonmark}}",
		},
		{
			name: "malformed template emits no enchantment",
			raw:  "{{Dragonmark|Lesser}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateDragonmark(tt.raw, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplateDragonmark(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseEnchantmentsDragonmark(t *testing.T) {
	want := []dataset.Enchantment{{
		Name:      "Minor Greater Dragonmark Enhancement",
		Amount:    "1",
		BonusType: "Enhancement",
		Notes:     new("This will increase the total number of Greater Dragonmarks you can use by 1. However, these additional Greater Dragonmarks will only take effect after the wielder rests."),
	}}

	got := ParseEnchantments("{{Dragonmark|Greater|Minor}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}
