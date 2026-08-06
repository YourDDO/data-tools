package parser

import (
	"reflect"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestParseEnchantmentsNotableTarget(t *testing.T) {
	want := []dataset.Enchantment{{
		Name:  "Notable Target",
		Notes: new("Once per minute, when you use Intimidate, you gain a +100% Profane bonus to threat generation with weapon strikes for 20 seconds."),
	}}

	got := ParseEnchantments("{{NotableTarget}}", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnchantments() = %#v, want %#v", got, want)
	}
}
