package domain

import "strings"

var compatibleAugmentTypes = map[string]map[string]struct{}{
	"red":       {"red": {}, "colorless": {}},
	"blue":      {"blue": {}, "colorless": {}},
	"yellow":    {"yellow": {}, "colorless": {}},
	"green":     {"green": {}, "blue": {}, "yellow": {}, "colorless": {}},
	"orange":    {"orange": {}, "red": {}, "yellow": {}, "colorless": {}},
	"purple":    {"purple": {}, "red": {}, "blue": {}, "colorless": {}},
	"colorless": {"colorless": {}},
}

// AugmentTypesCompatible reports whether an augment type can be installed in
// a slot type. Unknown types are compatible only with themselves.
func AugmentTypesCompatible(slotType, augmentType string) bool {
	slotType = strings.ToLower(strings.TrimSpace(slotType))
	augmentType = strings.ToLower(strings.TrimSpace(augmentType))
	if allowed, exists := compatibleAugmentTypes[slotType]; exists {
		_, accepted := allowed[augmentType]
		return accepted
	}
	return slotType != "" && slotType == augmentType
}
