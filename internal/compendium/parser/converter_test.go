package parser

import (
	"reflect"
	"testing"
	"yourddo-data-tools/internal/dataset"
)

func TestLGSAugments(t *testing.T) {
	tests := []struct {
		name     string
		fields   map[string]string
		expected []dataset.AugmentItem
	}{
		{
			name: "LGSAugments No Params",
			fields: map[string]string{
				"emptyaugments": "{{LGSAugments}}",
			},
			expected: []dataset.AugmentItem{
				{AugmentType: "Green Steel Epic Tier 1"},
				{AugmentType: "Green Steel Epic Tier 2"},
				{AugmentType: "Green Steel Epic Tier 3"},
				{AugmentType: "Green Steel Epic Tier Active"},
			},
		},
		{
			name: "LGSAugments With Param",
			fields: map[string]string{
				"emptyaugments": "{{LGSAugments|1}}",
			},
			expected: []dataset.AugmentItem{
				{AugmentType: "Green Steel Epic Tier 1"},
				{AugmentType: "Green Steel Epic Tier 2"},
				{AugmentType: "Green Steel Epic Tier 3"},
				{AugmentType: "Green Steel Epic Tier Active"},
				{AugmentType: "Fangs of Shavarath"},
			},
		},
		{
			name: "LGSAugments Singular Field",
			fields: map[string]string{
				"emptyaugment": "{{LGSAugments|(Clothing)}}",
			},
			expected: []dataset.AugmentItem{
				{AugmentType: "Green Steel Epic Tier 1"},
				{AugmentType: "Green Steel Epic Tier 2"},
				{AugmentType: "Green Steel Epic Tier 3"},
				{AugmentType: "Green Steel Epic Tier Active"},
				{AugmentType: "Fangs of Shavarath"},
			},
		},
		{
			name: "LGSAugments in Enchantments",
			fields: map[string]string{
				"enchantments": "{{LGSAugments}}",
			},
			expected: []dataset.AugmentItem{
				{AugmentType: "Green Steel Epic Tier 1"},
				{AugmentType: "Green Steel Epic Tier 2"},
				{AugmentType: "Green Steel Epic Tier 3"},
				{AugmentType: "Green Steel Epic Tier Active"},
			},
		},
		{
			name: "LGSAugments Nested in EmptyAugments",
			fields: map[string]string{
				"emptyaugments": "{{EmptyAugments|{{LGSAugments}}}}",
			},
			expected: []dataset.AugmentItem{
				{AugmentType: "Green Steel Epic Tier 1"},
				{AugmentType: "Green Steel Epic Tier 2"},
				{AugmentType: "Green Steel Epic Tier 3"},
				{AugmentType: "Green Steel Epic Tier Active"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ConvertItemToJSON(tt.name, tt.fields)
			if !reflect.DeepEqual(item.Augments, tt.expected) {
				t.Errorf("ConvertItemToJSON() Augments = %v, want %v", item.Augments, tt.expected)
			}
		})
	}
}

func TestLGSAugmentDrop(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected dataset.DropSourceData
	}{
		{
			name: "LGSAugment Named Params",
			raw:  "{{LGSAugment |altar = Invasion |pFocus = Air |sFocus = Earth |gem = Opposition |essence = Material }}",
			expected: dataset.DropSourceData{
				SourceType:        "LGSAugment",
				LGSAugmentAltar:   "Invasion",
				LGSAugmentPFocus:  "Air",
				LGSAugmentSFocus:  "Earth",
				LGSAugmentGem:     "Opposition",
				LGSAugmentEssence: "Material",
			},
		},
		{
			name: "LGSAugment Positional Params",
			raw:  "{{LGSAugment|Invasion|Air|Earth|Opposition|Material}}",
			expected: dataset.DropSourceData{
				SourceType:        "LGSAugment",
				LGSAugmentAltar:   "Invasion",
				LGSAugmentPFocus:  "Air",
				LGSAugmentSFocus:  "Earth",
				LGSAugmentGem:     "Opposition",
				LGSAugmentEssence: "Material",
			},
		},
		{
			name: "LGSAugment Mixed/Missing Params",
			raw:  "{{LGSAugment|altar=Invasion|pFocus=Air|essence=Material}}",
			expected: dataset.DropSourceData{
				SourceType:        "LGSAugment",
				LGSAugmentAltar:   "Invasion",
				LGSAugmentPFocus:  "Air",
				LGSAugmentEssence: "Material",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateLGSAugment(tt.raw)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseTemplateLGSAugment() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestGreenSteelCraftingPurchaseDrop(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected dataset.DropSourceData
	}{
		{
			name: "GSC Named Params",
			raw:  "{{GreenSteelCraftingPurchase |altar = Devastation |arrow = 1 |bone = 1 |shrapnel = 1 |stone = 1 |comms = 100 |legendary = 1 }}",
			expected: dataset.DropSourceData{
				SourceType:   "GreenSteelCraftingPurchase",
				GSCAltar:     "Devastation",
				GSCArrow:     "1",
				GSCBone:      "1",
				GSCShrapnel:  "1",
				GSCStone:     "1",
				GSCComms:     "100",
				GSCLegendary: "1",
			},
		},
		{
			name: "GSC Positional Params",
			raw:  "{{GreenSteelCraftingPurchase|Devastation|1|2|3|4|5|6|100|10|1}}",
			expected: dataset.DropSourceData{
				SourceType:   "GreenSteelCraftingPurchase",
				GSCAltar:     "Devastation",
				GSCArrow:     "1",
				GSCBone:      "2",
				GSCShrapnel:  "3",
				GSCChain:     "4",
				GSCStone:     "5",
				GSCScales:    "6",
				GSCComms:     "100",
				GSCRunes:     "10",
				GSCLegendary: "1",
			},
		},
		{
			name: "GSC Mixed/Missing Params",
			raw:  "{{GreenSteelCraftingPurchase|altar=Devastation|comms=100}}",
			expected: dataset.DropSourceData{
				SourceType: "GreenSteelCraftingPurchase",
				GSCAltar:   "Devastation",
				GSCComms:   "100",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTemplateGreenSteelCraftingPurchase(tt.raw)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseTemplateGreenSteelCraftingPurchase() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestCraftedAugment(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		wantCraftedIn   string
		wantLocation    string
		wantArea        string
		wantRaidItems   []string
		wantIngredients []dataset.CraftingRequirement
	}{
		{
			name:          "Cadence",
			raw:           "{{CraftedAugment|Cadence|Tattered Scrolls of the Broken One}}",
			wantCraftedIn: "Cauldron of Cadence",
			wantLocation:  "The Hut from Beyond",
			wantArea:      "The Harbor",
			wantRaidItems: []string{"Tattered Scrolls of the Broken One"},
			wantIngredients: []dataset.CraftingRequirement{
				{Name: "Thread of Fate", Quantity: new(50)},
				{Name: "Empty Soul Vessel", Quantity: new(1)},
				{Name: "Tattered Scrolls of the Broken One", Quantity: new(1)},
			},
		},
		{
			name:          "Soulforge",
			raw:           "{{CraftedAugment|Soulforge|The Broken Blade of Constellation|The Shattered Hilt of Constellation}}",
			wantCraftedIn: "Soulforge",
			wantLocation:  "Hall of Heroes",
			wantRaidItems: []string{"The Broken Blade of Constellation", "The Shattered Hilt of Constellation"},
			wantIngredients: []dataset.CraftingRequirement{
				{Name: "Thread of Fate", Quantity: new(50)},
				{Name: "Empty Soul Vessel", Quantity: new(1)},
				{Name: "The Broken Blade of Constellation", Quantity: new(1)},
				{Name: "The Shattered Hilt of Constellation", Quantity: new(1)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drop := parseTemplateCraftedAugment(tt.raw)
			if drop.CraftLocation != tt.wantCraftedIn || drop.VendorName != tt.wantCraftedIn {
				t.Fatalf("crafting station = (%q, %q), want %q", drop.CraftLocation, drop.VendorName, tt.wantCraftedIn)
			}
			if drop.VendorLocation != tt.wantLocation || drop.VendorArea != tt.wantArea {
				t.Fatalf("vendor location = (%q, %q), want (%q, %q)", drop.VendorLocation, drop.VendorArea, tt.wantLocation, tt.wantArea)
			}
			if !reflect.DeepEqual(drop.RaidItems, tt.wantRaidItems) {
				t.Fatalf("raid items = %#v, want %#v", drop.RaidItems, tt.wantRaidItems)
			}
			if !reflect.DeepEqual(drop.Ingredients, tt.wantIngredients) {
				t.Fatalf("ingredients = %#v, want %#v", drop.Ingredients, tt.wantIngredients)
			}

			augment := ConvertAugmentToJSON("Test Augment", map[string]string{"droplocation": tt.raw})
			if augment.CraftedIn != tt.wantCraftedIn {
				t.Fatalf("craftedIn = %q, want %q", augment.CraftedIn, tt.wantCraftedIn)
			}
			if len(augment.Requirements) != len(tt.wantIngredients) {
				t.Fatalf("requirements = %#v, want %d entries", augment.Requirements, len(tt.wantIngredients))
			}
			for index, requirement := range augment.Requirements {
				want := tt.wantIngredients[index]
				if requirement.Title != want.Name || !reflect.DeepEqual(requirement.Quantity, want.Quantity) {
					t.Fatalf("requirement %d = %#v, want title %q quantity %v", index, requirement, want.Name, *want.Quantity)
				}
			}
		})
	}
}

func TestExtractSetBonusesHonorsTemplateControlParameters(t *testing.T) {
	t.Parallel()
	item := ConvertItemToJSON("Mysterious Ring", map[string]string{
		"name":     "Mysterious Ring",
		"itemsets": "{{ItemSetList|The Desert's Biting Sands|The Desert's Burning Sun|The Desert's Starless Nights|The Desert's Writhing Storm|True}}",
	})
	want := []string{
		"The Desert's Biting Sands",
		"The Desert's Burning Sun",
		"The Desert's Starless Nights",
		"The Desert's Writhing Storm",
	}
	if len(item.SetBonus) != len(want) {
		t.Fatalf("set bonuses = %#v, want %v", item.SetBonus, want)
	}
	for index, name := range want {
		if item.SetBonus[index].Name != name {
			t.Fatalf("set bonuses = %#v, want %v", item.SetBonus, want)
		}
	}

	singular := extractSetBonusesFromText("{{ItemSet|Vol's Influence|True|False}}")
	if len(singular) != 1 || singular[0].Name != "Vol's Influence" {
		t.Fatalf("singular item set = %#v", singular)
	}
}

func TestParseTemplatePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want dataset.PriceData
	}{
		{
			name: "all currencies",
			raw:  "{{Price|1000|200|30|4}}",
			want: dataset.PriceData{Platinum: "1000", Gold: "200", Silver: "30", Copper: "4"},
		},
		{
			name: "sparse positional currencies",
			raw:  "{{Price|1000|||4}}",
			want: dataset.PriceData{Platinum: "1000", Copper: "4"},
		},
		{
			name: "whitespace around template and values",
			raw:  "  {{Price | 1000 | 200 | 30 | 4 }}  ",
			want: dataset.PriceData{Platinum: "1000", Gold: "200", Silver: "30", Copper: "4"},
		},
		{
			name: "empty template",
			raw:  "{{Price}}",
			want: dataset.PriceData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTemplatePrice(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTemplatePrice(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
