package parser

import (
	"reflect"
	"strings"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestMasterExclusionReason(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		title string
		raw   string
		want  string
	}{
		{name: "pre update", title: "Old Item (Pre U10)", raw: "{{Item|name=Old Item}}", want: "pre-update item"},
		{name: "discontinued", title: "Old Item", raw: "{{Item|name=Old Item}}{{Discontinued|U10}}", want: "discontinued item"},
		{name: "starter", title: "Starter Item", raw: "{{Item|name=Starter Item}}{{Starter}}", want: "starter item"},
		{name: "ordinary", title: "Current Item", raw: "{{Item|name=Current Item}}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := MasterExclusionReason(test.title, test.raw); got != test.want {
				t.Fatalf("MasterExclusionReason() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestHasDiscontinuedDropLocation(t *testing.T) {
	t.Parallel()
	raw := "{{Item|name=Old Item|type=Ring|droplocation={{Discontinued |legacy}}}}"
	if !HasDiscontinuedDropLocation(raw) {
		t.Fatal("spaced Discontinued drop-location template was not excluded")
	}
}

func TestParseAugmentRecordIdentifiesUnrenderedNestedFieldTemplate(t *testing.T) {
	t.Parallel()
	raw := "{{Augment|name=Nested Level Augment|color=Red|minlevel={{MinimumLevel|2}}}}"
	_, err := ParseAugmentRecord("Nested Level Augment", raw)
	if err == nil || !strings.Contains(err.Error(), `nested template "MinimumLevel"`) ||
		!strings.Contains(err.Error(), "was not rendered by the Compendium API") ||
		!strings.Contains(err.Error(), `raw value "{{MinimumLevel|2}}"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestParseItemRecordIdentifiesUnclosedNestedTemplate(t *testing.T) {
	t.Parallel()
	raw := "{{Item|name=Broken Item|type=Ring|enchantments={{Ability|Strength|2"
	_, err := ParseItemRecord("Broken Item", raw)
	if err == nil || !strings.Contains(err.Error(), `unclosed nested template "Ability"`) ||
		!strings.Contains(err.Error(), "at byte") || !strings.Contains(err.Error(), "near") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseItemRecordIdentifiesExtraBraceAfterNestedTemplate(t *testing.T) {
	t.Parallel()
	raw := "{{Template:Item|name=Perfected Golden Guile|type=Necklace|enchantments={{Deception|new Improved|21}}}|description=Test}}"
	_, err := ParseItemRecord("Perfected Golden Guile", raw)
	if err == nil || !strings.Contains(err.Error(), `nested template "Deception"`) ||
		!strings.Contains(err.Error(), "source wikitext contains an extra '}'") ||
		!strings.Contains(err.Error(), "browser renderer may tolerate") ||
		!strings.Contains(err.Error(), "at byte") || !strings.Contains(err.Error(), "near") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseItemRecordPreservesMaterialTemplateFields(t *testing.T) {
	t.Parallel()
	raw := `{{Template:Material
|name = Pristine Gem
|type = Gem
|bagtype = Augment
|bagicon = Custom Bag
|use = [[Crafting]]
|rarity = Rare
|collectors = [[Gem Collector]]
|notes = Valuable material.
|bagstacksize = 1,000
|inventorystacksize = 100
|options = {{ItemOptions|test}}
|doubleclickpurchase = {{VendorSale|Gem Merchant}}
|otherversions = [[Flawed Gem]]
|storedescription = A particularly pristine gem.
}}`

	item, err := ParseItemRecord("Pristine Gem", raw)
	if err != nil {
		t.Fatalf("ParseItemRecord() error = %v", err)
	}
	want := dataset.ItemData{
		PageTitle:           "Pristine Gem",
		Name:                "Pristine Gem",
		Type:                "Gem",
		BagType:             "Augment",
		BagIcon:             "Custom Bag",
		Use:                 "[[Crafting]]",
		Rarity:              "Rare",
		Collectors:          "[[Gem Collector]]",
		Notes:               "Valuable material.",
		BagStackSize:        "1,000",
		InventoryStackSize:  "100",
		Options:             "{{ItemOptions|test}}",
		DoubleClickPurchase: "{{VendorSale|Gem Merchant}}",
		OtherVersions:       "[[Flawed Gem]]",
		StoreDescription:    "A particularly pristine gem.",
	}
	if !reflect.DeepEqual(item, want) {
		t.Fatalf("ParseItemRecord() = %#v, want %#v", item, want)
	}
}
