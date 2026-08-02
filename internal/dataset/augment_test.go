package dataset

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPriceModifierUsesEssenceCraftingName(t *testing.T) {
	t.Parallel()
	value := 4
	data, err := json.Marshal(PriceModifierOut{EssenceCrafting: &value})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != `{"essenceCrafting":4}` {
		t.Fatalf("JSON = %s", got)
	}
	if strings.Contains(string(data), "cannithCrafting") {
		t.Fatal("legacy Cannith Crafting JSON key was emitted")
	}
}
