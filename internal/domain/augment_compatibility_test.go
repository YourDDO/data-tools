package domain

import "testing"

func TestAugmentTypesCompatible(t *testing.T) {
	for _, test := range []struct {
		slotType    string
		augmentType string
		want        bool
	}{
		{"Green", "Blue", true},
		{"Green", "Yellow", true},
		{"Orange", "Red", true},
		{"Purple", "Blue", true},
		{"Red", "Blue", false},
		{"Colorless", "Colorless", true},
		{"unknown", "unknown", true},
		{"unknown", "other", false},
	} {
		t.Run(test.slotType+"/"+test.augmentType, func(t *testing.T) {
			if got := AugmentTypesCompatible(test.slotType, test.augmentType); got != test.want {
				t.Fatalf("AugmentTypesCompatible(%q, %q) = %t, want %t", test.slotType, test.augmentType, got, test.want)
			}
		})
	}
}
