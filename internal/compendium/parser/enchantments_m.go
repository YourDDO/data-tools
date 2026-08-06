package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"yourddo-data-tools/internal/dataset"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func parseTemplateMurderous(rawMValue string) *dataset.Enchantment {
	const prefix = "{{Murderous|"
	const suffix = "}}"
	const baseName = "Murderous"

	if !strings.HasPrefix(rawMValue, prefix) || !strings.HasSuffix(rawMValue, suffix) {
		return nil
	}

	paramList := rawMValue[len(prefix) : len(rawMValue)-len(suffix)]
	parts := strings.Split(paramList, "|")

	// Docs: (Type)|(Title)

	// 1. Type (Required, Index 0)
	if len(parts) < 1 {
		return nil
	}
	mType := stripBrackets(parts[0])
	if mType == "" {
		return nil
	}

	var name string
	var title string

	// 2. Title (Optional, Index 1)
	if len(parts) >= 2 {
		title = stripBrackets(parts[1])
	}

	// --- MAPPING AND NAME FORMATTING ---

	// 1. Determine the base damage type for classification (Element field)
	damageType := mType
	if dt, ok := murderousTypeToDamage[mType]; ok {
		damageType = dt
	}

	// 2. Determine the final display name
	if title != "" {
		name = title // Use custom title
	} else {
		// Default name format: "Murderous [Type]" (e.g., "Murderous Point")
		name = baseName + " " + mType
	}

	return &dataset.Enchantment{
		Name:    name,
		Element: damageType, // Store Piercing/Slashing/Bludgeoning here
		// All other fields remain empty.
	}
}

func parseTemplateMagicalEfficiency(rawMEValue string) *dataset.Enchantment {
	const prefix = "{{MagicalEfficiency|"
	const suffix = "}}"
	const baseName = "Spell Point Multiplier" // Required Name
	const bonusType = "Enhancement"           // Assumed default based on example

	if !strings.HasPrefix(rawMEValue, prefix) || !strings.HasSuffix(rawMEValue, suffix) {
		return nil
	}

	paramList := rawMEValue[len(prefix) : len(rawMEValue)-len(suffix)]
	parts := strings.Split(paramList, "|")

	// Docs: (Enhancement Amount)

	// 1. Enhancement Amount (Required, Index 0)
	if len(parts) < 1 {
		return nil
	}
	amountRaw := stripBrackets(parts[0])
	if amountRaw == "" {
		return nil
	}

	// --- CRITICAL AMOUNT FORMATTING ---
	// The amount must be stored as a negative percentage (e.g., 5 -> -5%)
	amount := ""
	if num, err := strconv.Atoi(amountRaw); err == nil {
		amount = fmt.Sprintf("-%d%%", num)
	} else {
		// If conversion fails (e.g., amount is 'I', 'V'), use raw value with a negative prefix
		amount = "-" + amountRaw + "%"
	}
	// ------------------------------------

	return &dataset.Enchantment{
		Name:      baseName,
		Amount:    amount,
		BonusType: bonusType,
		// No other fields are needed.
	}
}

// parseTemplateMeridianFragment parses
// `{{MeridianFragment|Universal Spell Power|Maximum Stacks|Duration}}`.
func parseTemplateMeridianFragment(raw string) *dataset.Enchantment {
	const (
		template = "MeridianFragment"
		prefix   = "{{" + template
		suffix   = "}}"
	)

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	inner := s[len(prefix) : len(s)-len(suffix)]
	if inner != "" {
		var hasParams bool
		inner, hasParams = strings.CutPrefix(inner, "|")
		if !hasParams {
			return nil
		}
	}

	amount, maximumStacks, duration := "8", "3", "20"
	parts := splitParams(inner)
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		amount = strings.TrimSpace(stripBrackets(parts[0]))
	}
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		maximumStacks = strings.TrimSpace(stripBrackets(parts[1]))
	}
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		duration = strings.TrimSpace(stripBrackets(parts[2]))
	}

	amountValue, err := strconv.ParseFloat(amount, 64)
	if err != nil || math.IsNaN(amountValue) || math.IsInf(amountValue, 0) {
		return nil
	}
	stacks, err := strconv.Atoi(maximumStacks)
	if err != nil || stacks <= 0 {
		return nil
	}
	durationValue, err := strconv.ParseFloat(duration, 64)
	if err != nil || math.IsNaN(durationValue) || math.IsInf(durationValue, 0) || durationValue <= 0 {
		return nil
	}

	notes := fmt.Sprintf("Once every three seconds when you take physical damage, you gain a +%s Psionic bonus to Universal Spell Power. This effect can stack up to %s times, and each stack lasts for %s seconds.", amount, maximumStacks, duration)
	return &dataset.Enchantment{
		Name:      "Spell Power: Universal",
		Amount:    amount,
		BonusType: "Psionic",
		Notes:     new(notes),
	}
}

func parseTemplateMarksmanship(rawMSValue string) []*dataset.Enchantment {
	const prefix = "{{Marksmanship|"
	const suffix = "}}"

	var enchType = "Default" // Assume Default if blank

	if strings.TrimSpace(rawMSValue) == "{{Marksmanship}}" {
		// Argument-less call defaults to the "Default" lookup
	} else if strings.HasPrefix(rawMSValue, prefix) && strings.HasSuffix(rawMSValue, suffix) {
		paramList := rawMSValue[len(prefix) : len(rawMSValue)-len(suffix)]
		parts := strings.Split(paramList, "|")

		// 1. Type (Index 0)
		if len(parts) >= 1 {
			val := strings.TrimSpace(parts[0])
			if val != "" {
				enchType = cases.Title(language.English).String(strings.ToLower(val)) // Normalize to "Greater", "New Normal", or "Default"
			}
		}
	} else {
		return nil // Invalid format
	}

	// Lookup the fixed values
	lookup, exists := marksmanshipLookup[enchType]
	if !exists {
		lookup = marksmanshipLookup["Default"] // Fallback
	}

	var enchantments []*dataset.Enchantment

	// --- 1. Ranged Attack Rolls Enhancement ---

	enchantments = append(enchantments, &dataset.Enchantment{
		Name:      "Attack Rolls (Ranged)",
		Amount:    lookup.AttackAmount,
		BonusType: lookup.BonusType,
	})

	// --- 2. Ranged Damage Enhancement ---
	enchantments = append(enchantments, &dataset.Enchantment{
		Name:      "Damage (Ranged)",
		Amount:    lookup.DamageAmount,
		BonusType: lookup.BonusType,
	})

	return enchantments
}

func parseTemplateMemoryOfButchery() *dataset.Enchantment {
	const templateName = "Memory of Butchery"

	// Documentation: No arguments, sets the Name.

	return &dataset.Enchantment{
		Name: templateName,
		// All other fields remain empty.
	}
}

func parseTemplateMemoryOfAnimatedObjects() []*dataset.Enchantment {
	return []*dataset.Enchantment{
		{Name: "Spell Power: Repair", Amount: "171", BonusType: "Equipment"},
		{Name: "Spell Power: Rust", Amount: "171", BonusType: "Equipment"},
		{Name: "Spell Critical Chance: Repair", Amount: "24", BonusType: "Equipment"},
		{Name: "Spell Critical Chance: Rust", Amount: "24", BonusType: "Equipment"},
		{Name: "Spell Critical Damage: Repair", Amount: "25", BonusType: "Enhancement", Element: "Repair"},
		{Name: "Spell Critical Damage: Rust", Amount: "25", BonusType: "Enhancement", Element: "Rust"},
	}
}

func parseTemplateMemoryOfBinding() *dataset.Enchantment {
	const templateName = "Memory of Binding"

	// Documentation: No arguments, sets the Name.

	return &dataset.Enchantment{
		Name: templateName,
		// All other fields remain empty.
	}
}

func parseTemplateMemoryOfShatteredLife() *dataset.Enchantment {
	const templateName = "Memory of Shattered Life"

	// Documentation: No arguments, sets the Name.

	return &dataset.Enchantment{
		Name: templateName,
		// All other fields remain empty.
	}
}

func parseTemplateMaiming(rawMValue string) *dataset.Enchantment {
	const (
		templateName = "Maiming"
		prefix       = "{{" + templateName + "|"
		suffix       = "}}"
	)

	rawMValue = strings.TrimSpace(rawMValue)
	if rawMValue == "{{"+templateName+"}}" {
		return &dataset.Enchantment{
			Name:    templateName,
			Element: "Untyped",
			Notes:   new("On critical hit: x2 1d6, x3 2d6, or x4 3d6 untyped damage."),
		}
	}
	if !strings.HasPrefix(rawMValue, prefix) || !strings.HasSuffix(rawMValue, suffix) {
		return nil
	}

	parts := strings.Split(rawMValue[len(prefix):len(rawMValue)-len(suffix)], "|")
	maimingType := ""
	if len(parts) > 0 {
		maimingType = strings.ToLower(stripBrackets(parts[0]))
	}
	amount := ""
	if len(parts) > 1 {
		amount = stripBrackets(parts[1])
	}

	switch maimingType {
	case "", "normal":
		return &dataset.Enchantment{
			Name:    templateName,
			Element: "Untyped",
			Notes:   new("On critical hit: x2 1d6, x3 2d6, or x4 3d6 untyped damage."),
		}
	case "greater":
		return &dataset.Enchantment{
			Name:    "Greater " + templateName,
			Element: "Untyped",
			Notes:   new("On critical hit: x2 4d6, x3 12d6, or x4 16d6 untyped damage."),
		}
	case "augment":
		return &dataset.Enchantment{
			Name:    "Greater " + templateName,
			Element: "Untyped",
			Notes:   new("On critical hit: x2 8d6, x3 12d6, or x4 16d6 untyped damage."),
		}
	case "new":
		if amount == "" {
			return nil
		}
		return &dataset.Enchantment{
			Name:    templateName + " " + amount,
			Amount:  amount + "d8",
			Element: "Untyped",
			Notes:   new("On critical hit: untyped damage."),
		}
	case "weapon":
		if amount == "" {
			return nil
		}
		return &dataset.Enchantment{
			Name:  "Weapon's " + templateName + " Effect +" + amount,
			Notes: new("Does additional damage on critical hits."),
		}
	default:
		// The template's default branch renders regular Maiming for unrecognised types.
		return &dataset.Enchantment{
			Name:    templateName,
			Element: "Untyped",
			Notes:   new("On critical hit: x2 1d6, x3 2d6, or x4 3d6 untyped damage."),
		}
	}
}

func parseTemplateMeltscale(rawMSValue string) *dataset.Enchantment {
	const templateName = "Meltscale"
	const prefix = "{{" + templateName + "}}"
	const damageDice = "15d6"
	const damageElement = "Acid"

	// This template has no arguments, so we check for the exact full string
	if strings.TrimSpace(rawMSValue) != prefix {
		return nil
	}

	// Documentation: No arguments.

	return &dataset.Enchantment{
		Name: templateName,
		// Use Amount for the primary damage dice
		Amount: damageDice,
		// Use Element for the damage type
		Element: damageElement,
	}
}

// Template:Metalline
// Usage per docs: {{Metalline|(Type)|(Metal)|(Title)}}
//   - Type: blank/None (basic → bypasses ALL metal DR), "Legendary" (also bypasses ALL), or "Single" (requires Metal param).
//   - Metal: when Type==Single, indicates which metal DR is bypassed. Accept common aliases.
//   - Title: Ignored. We standardize output to one or more effects with names:
//     "Damage Reduction Bypass: <Metal>"
//
// Metals covered (from documentation): Adamantine, Alchemical Silver, Byeshk, Cold Iron, Mithril.
func parseTemplateMetalline(raw string) []*dataset.Enchantment {
	const prefix = "{{Metalline"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	// Extract inner
	inner := strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix)
	inner = strings.TrimSpace(inner)
	if after, ok := strings.CutPrefix(inner, "|"); ok {
		inner = after
	}

	// Split top-level params simply; Metalline params do not nest templates typically
	parts := []string{}
	if inner != "" {
		parts = strings.Split(inner, "|")
	}

	// Normalize metals list and helpers
	allMetals := []string{"Adamantine", "Alchemical Silver", "Byeshk", "Cold Iron", "Mithril"}

	// Alias map for input normalization
	alias := map[string]string{
		"adamantine":        "Adamantine",
		"silver":            "Alchemical Silver",
		"alchemical silver": "Alchemical Silver",
		"byeshk":            "Byeshk",
		"cold iron":         "Cold Iron",
		"mithril":           "Mithril",
	}

	// Helper to emit unique in deterministic order
	addEffects := func(metals []string) []*dataset.Enchantment {
		seen := map[string]bool{}
		out := make([]*dataset.Enchantment, 0, len(metals))
		for _, m := range metals {
			norm := strings.TrimSpace(m)
			if norm == "" {
				continue
			}
			// normalize to canonical display via alias table
			key := strings.ToLower(norm)
			if v, ok := alias[key]; ok {
				norm = v
			} else {
				// try to title-case generic input
				norm = cases.Title(language.English).String(key)
			}
			if !seen[norm] {
				seen[norm] = true
				out = append(out, &dataset.Enchantment{Name: "Damage Reduction Bypass: " + norm, Element: norm})
			}
		}
		return out
	}

	// Determine behavior from Type
	typ := ""
	if len(parts) >= 1 {
		typ = stripBrackets(parts[0])
	}
	t := strings.ToLower(strings.TrimSpace(typ))

	switch t {
	case "", "none", "basic", "legendary":
		// Emit all metal bypasses
		return addEffects(allMetals)
	case "single":
		// Read Metal parameter; allow multiple separated by commas or '/'
		metalParam := ""
		if len(parts) >= 2 {
			metalParam = stripBrackets(parts[1])
		}
		metalParam = strings.TrimSpace(metalParam)
		if metalParam == "" {
			return nil
		}
		// Split on commas and slashes
		split := func(s string) []string {
			s = strings.ReplaceAll(s, "/", ",")
			segs := strings.Split(s, ",")
			for i := range segs {
				segs[i] = strings.TrimSpace(segs[i])
			}
			return segs
		}
		return addEffects(split(metalParam))
	default:
		// Unrecognized type; safest default is to treat like basic Metalline
		return addEffects(allMetals)
	}
}

// Template: MagmaSurgeGuard
// Usage: {{MagmaSurgeGuard|(Type)}}
// - Type: Normal (Blank), Legendary
func parseTemplateMagmaSurgeGuard(raw string) *dataset.Enchantment {
	const prefix = "{{MagmaSurgeGuard"
	const suffix = "}}"
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	inner := s[len(prefix) : len(s)-len(suffix)]
	if after, ok := strings.CutPrefix(inner, "|"); ok {
		inner = after
	}

	parts := strings.Split(inner, "|")
	magmaType := ""
	if len(parts) > 0 {
		magmaType = strings.TrimSpace(parts[0])
	}

	name := "Magma Surge Guard"
	if strings.EqualFold(magmaType, "Legendary") {
		name = "Legendary Magma Surge Guard"
	}

	return &dataset.Enchantment{
		Name:  name,
		Notes: new("When the wearer of this item is successfully attacked in melee, superheated magma occasionally surges to the surface, slowing an enemy down and inflicting massive fire damage over time."),
	}
}

// Template: MagicalNull
// Usage: {{MagicalNull}}
func parseTemplateMagicalNull(raw string) *dataset.Enchantment {
	const prefix = "{{MagicalNull"
	const suffix = "}}"
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	return &dataset.Enchantment{
		Name:      "Arcane Spell Failure",
		BonusType: "Penalty",
		Notes:     new("This nullcloth that this is made from absorbs spell energies making it difficult for all spell casters, even Diving casters, to complete their spells. +15% Spell Failure chance."),
	}
}

func parseTemplateMetalFatigue(raw string) *dataset.Enchantment {
	const template = "MetalFatigue"
	const prefix = "{{" + template
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Metal Fatigue"
	notes := "When you are damaged there is a small chance that you will become Exhausted."

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

func parseTemplateMakersTouch() *dataset.Enchantment {
	name := "Maker's Touch"
	notes := "Casting a Repair spell on yourself of allies leaves a lingering defensive buff that increases the AC and Physical Resistance Rating of your target by +3 for 12 seconds."

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

func parseTemplateMelodicGuard(raw string) *dataset.Enchantment {
	const template = "MelodicGuard"
	const prefix = "{{" + template
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Melodic Guard"
	notes := "Each time you are hit in melee combat there is a chance that your attacker will be overcome with an urge to dance."

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

func parseTemplateMindDrain(raw string) *dataset.Enchantment {
	const template = "MindDrain"
	const prefix = "{{" + template
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Spell Points"
	bonusType := "Penalty"
	amount := "-5%"
	notes := "This item reduces your maximum spell points by 5% while equipped."

	return &dataset.Enchantment{
		Name:      name,
		BonusType: bonusType,
		Amount:    amount,
		Notes:     new(notes),
	}
}

func parseTemplateMortalStrike(raw string) *dataset.Enchantment {
	const prefix = "{{MortalStrike"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Mortal Strike"
	notes := "On an attack roll of 20 which is confirmed as a critical hit this weapon triggers the Slay Living spell and attempts to instantly snuff out the life force of the enemy."

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

// parseTemplatePlanarConflux parses `{{PlanarConflux}}`.

func parseTemplateMotherNightsEmbrace(raw string) *dataset.Enchantment {
	const prefix = "{{MotherNightsEmbrace"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix))
	if after, ok := strings.CutPrefix(inner, "|"); ok {
		inner = after
	}

	amount := ""
	if inner != "" {
		parts := strings.Split(inner, "|")
		if len(parts) >= 1 {
			amount = strings.TrimSpace(parts[0])
		}
	}

	name := "Mother Night's Embrace"
	notes := "This weapon is unholy and imbued with one of the two deities of Barovia - the Mother Night. This weapon is evil, dealing an additional 3d6 evil damage on each hit."

	if amount != "" {
		bonusType := "bonus"
		if strings.HasPrefix(amount, "-") {
			bonusType = "penalty"
		}
		// If amount doesn't have a sign, it's a bonus by default.
		if !strings.HasPrefix(amount, "+") && !strings.HasPrefix(amount, "-") {
			amount = "+" + amount
		}

		notes += fmt.Sprintf(" In addition, the weapon grants a %s %s to its enhancement bonus.", amount, bonusType)
	}

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

// parseTemplateRighteous parses `{{Righteous}}`.

func parseTemplateMasterwork(raw string) *dataset.Enchantment {
	const prefix = "{{Masterwork"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Masterwork"
	notes := "This weapon is more finely crafted than normal, providing a +1 enhancement bonus on attack rolls."

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

// parseTemplateCompletedWeapon parses `{{CompletedWeapon}}`.

func parseTemplateMalleable(raw string) *dataset.Enchantment {
	const prefix = "{{Malleable"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Malleable"
	notes := "This item has a magical aura of malleability and can be combined with any item that has the Fusible property to create a new, more powerful fusion of the two items."

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

// parseTemplateFusible parses `{{Fusible}}`.

func parseTemplateMindTurbulence(raw string) *dataset.Enchantment {
	const prefix = "{{MindTurbulence"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Conecentration"
	notes := "This item fills your mind with chaos, disrupting your thoughts and causing a -10 Concentration penalty."

	return &dataset.Enchantment{
		Name:      name,
		Amount:    "-10",
		BonusType: "Penalty",
		Notes:     new(notes),
	}
}

// parseTemplateLifedrinker parses `{{Lifedrinker}}`.

// parseTemplateMagmaSurge parses `{{MagmaSurge|(Type)|(Enhancement Amount)}}`.
func parseTemplateMagmaSurge(raw string) *dataset.Enchantment {
	const prefix = "{{MagmaSurge"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	content := strings.TrimPrefix(s, prefix)
	content = strings.TrimSuffix(content, suffix)

	var magmaType, enhancementAmount string
	if after, ok := strings.CutPrefix(content, "|"); ok {
		parts := strings.Split(after, "|")
		magmaType = strings.TrimSpace(parts[0])
		if len(parts) >= 2 {
			enhancementAmount = strings.TrimSpace(parts[1])
		}
	}

	const defaultNotes = "This weapon stores the immeasurable heat of the planet's molten mantle. When this weapon is used, superheated magma occasionally surges to the surface, slowing an enemy down and inflicting massive fire damage over time."

	switch strings.ToLower(magmaType) {
	case "legendary":
		return &dataset.Enchantment{
			Name:  "Legendary Magma Surge",
			Notes: new(defaultNotes),
		}
	case "damage":
		amount, err := strconv.Atoi(enhancementAmount)
		if err != nil {
			return nil
		}

		notes := fmt.Sprintf(
			"Strikes with this weapon have a small chance to call forth a surge of superheated magma, dealing %sd20+%d fire damage.",
			enhancementAmount,
			(amount+1)*9,
		)
		return &dataset.Enchantment{
			Name:  "Magma Surge +" + enhancementAmount,
			Notes: new(notes),
		}
	default:
		return &dataset.Enchantment{
			Name:  "Magma Surge",
			Notes: new(defaultNotes),
		}
	}
}

// parseTemplateWrathOfTheZealot parses `{{WrathOfTheZealot}}`.

func parseTemplateMedusaFury(raw string) *dataset.Enchantment {
	const prefix = "{{MedusaFury"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	name := "Medusa Fury"
	notes := "When you fall below 25% of your maximum hit points you will enter a constant Medusa Fury until your hit points return to 25% or greater. Medusa Fury grants a +4 morale bonus to Strength and Constitution, a 5% morale bonus to your chance to doublestrike, and a -25% penalty to fortifications."

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

// parseTemplateStealerOfSouls parses `{{StealerOfSouls|Soul Count|Type}}`.

func parseTemplateMindTear(raw string) *dataset.Enchantment {
	return parseSimpleTemplate(raw, "{{MindTear", "Mind Tear", "This weapon tears at the identity of your foes, reducing their MRR and Spell Power.")
}

func parseTemplateMindcleaver(raw string) *dataset.Enchantment {
	return parseSimpleTemplate(raw, "{{Mindcleaver", "Mindcleaver", "This blade is made of pure force and is surprisingly light to the touch. It bypasses the Incorporeal chances of Ethereal monsters innately.")
}

// parseTemplateTheMoralCompass parses `{{TheMoralCompass}}`.

func parseTemplateMonkPath(raw string) *dataset.Enchantment {
	const prefix = "{{MonkPath"
	const suffix = "}}"

	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return nil
	}

	content := strings.TrimPrefix(s, prefix)
	content = strings.TrimSuffix(content, suffix)

	stance := ""
	if after, ok := strings.CutPrefix(content, "|"); ok {
		stance = strings.TrimSpace(after)
	}

	var name, notes string
	if strings.EqualFold(stance, "sun") {
		name = "Path of the Fire Dragon"
		notes = "While wearing this item and in sun stance, you gain a +54 Equipment bonus to Fire Spell Power, which increases damage from spells and fire finishing moves such as Breath of the Fire Dragon."
	} else if strings.EqualFold(stance, "mountain") {
		name = "Path of the Guarding Stone"
		notes = "While wearing this item and in mountain stance, there is a chance you will be protected by a Stoneskin spell when enemies strike you."
	} else {
		// Default to mountain if stance is unknown or missing, based on the switch logic usually having a default or first case
		// But here it seems safer to just return a basic name if unknown
		name = "Monk Path"
		notes = "Please add which monk stance is being used (Sun, Mountain)"
	}

	return &dataset.Enchantment{
		Name:  name,
		Notes: new(notes),
	}
}

// parseTemplatePathoftheGuardingStone parses `{{PathoftheGuardingStone}}`.

func parseTemplateMissingParts() *dataset.Enchantment {
	return &dataset.Enchantment{
		Name:  "Missing Parts",
		Notes: new("Toven's Hammer appears to have been damaged during the fight. You think that you might be able to partially repair it if you had a elemental motion fixation device."),
	}
}

// parseTemplateOccasionalOvercooling handles Template:OccasionalOvercooling

func parseTemplateMemoryofChainsBroken() *dataset.Enchantment {
	return &dataset.Enchantment{
		Name:  "Memory of Chains Broken",
		Notes: new("Your hits have a 5% chance to Paralyze enemies for 5 seconds."),
	}
}

// parseTemplateOrlassksPrison handles Template:OrlassksPrison
