package essencecrafting

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
)

var placementAliases = map[string]string{
	"Weapon": "weapon", "Runearm": "rune-arm", "Orb": "orb", "Armor": "armor", "Belt": "belt", "Waist": "belt",
	"Boots": "boots", "Feet": "boots", "Bracers": "bracers", "Wrists": "bracers", "Cloak": "cloak", "Gloves": "gloves",
	"Hand": "gloves", "Hands": "gloves", "Goggles": "goggles", "Eyes": "goggles", "Head": "head", "Headgear": "head",
	"Necklace": "necklace", "Neck": "necklace", "Ring": "ring", "Rings": "ring", "Fingers": "ring", "Trinket": "trinket", "Shield": "shield",
}

var percentEffects = map[string]struct{}{
	"Acid Absorption": {}, "Acid Spell Critical Chance": {}, "Cold Absorption": {}, "Cold Spell Critical Chance": {}, "Concealment": {}, "Dodge": {},
	"Doubleshot": {}, "Doublestrike": {}, "Electric Absorption": {}, "Electric Spell Critical Chance": {}, "Fire Absorption": {}, "Fire Spell Critical Chance": {},
	"Force Absorption": {}, "Force Spell Critical Chance": {}, "Fortification": {}, "Fortification Bypass": {}, "Light Absorption": {}, "Light Spell Critical Chance": {},
	"Melee Attack Speed": {}, "Melee Threat": {}, "Movement Speed": {}, "Negative Absorption": {}, "Negative Spell Critical Chance": {}, "Offhand Shield Bash Chance": {},
	"Poison Absorption": {}, "Poison Spell Critical Chance": {}, "Positive Spell Critical Chance": {}, "Ranged Alacrity": {}, "Repair Spell Critical Chance": {},
	"Ranged Threat": {}, "Sonic Absorption": {}, "Sonic Spell Critical Chance": {}, "Spell Threat": {},
}

var dicePattern = regexp.MustCompile(`^d([1-9][0-9]*)$`)

func validateSourceEnhancement(context string, input sourceEnhancement) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%s: name is required", context)
	}
	if input.MinimumLevel < minimumItemLevel || input.MinimumLevel > maximumItemLevel {
		return fmt.Errorf("%s: minItemLevel %d is outside %d-%d", context, input.MinimumLevel, minimumItemLevel, maximumItemLevel)
	}
	if input.Bound == nil || input.Unbound == nil {
		return fmt.Errorf("%s: bound and unbound recipes are required", context)
	}
	if len(input.Prefix)+len(input.Suffix)+len(input.Extra) == 0 {
		return fmt.Errorf("%s: at least one placement is required", context)
	}
	return nil
}

func normalizePlacements(input sourceEnhancement) ([]contracts.EssenceCraftingPlacement, error) {
	values := []struct {
		position string
		slots    []string
	}{{"prefix", input.Prefix}, {"suffix", input.Suffix}, {"extra", input.Extra}}
	result := make([]contracts.EssenceCraftingPlacement, 0, 3)
	seenTriples := map[string]struct{}{}
	for _, value := range values {
		if len(value.slots) == 0 {
			continue
		}
		categories := make([]string, 0, len(value.slots))
		for _, raw := range value.slots {
			category, exists := placementAliases[strings.TrimSpace(raw)]
			if !exists {
				return nil, fmt.Errorf("unsupported %s position item category %q", value.position, raw)
			}
			triple := value.position + "\x00" + category
			if _, exists := seenTriples[triple]; exists {
				return nil, fmt.Errorf("duplicate normalized placement %s/%s", value.position, category)
			}
			seenTriples[triple] = struct{}{}
			categories = append(categories, category)
		}
		sort.Strings(categories)
		result = append(result, contracts.EssenceCraftingPlacement{Position: value.position, ItemCategoryIDs: categories})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("at least one normalized placement is required")
	}
	return result, nil
}

func transformScaledEffect(parent sourceEnhancement, input sourceEffect) (contracts.EssenceCraftingEffect, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return contracts.EssenceCraftingEffect{}, fmt.Errorf("name is required")
	}
	bonus := strings.TrimSpace(input.Bonus)
	result := contracts.EssenceCraftingEffect{ID: opaqueID("effect", strings.ToLower(bonus)+"\x00"+name), DisplayName: name, BonusTypeID: bonusID(bonus)}
	if len(input.Modifiers) == 0 {
		if (parent.Name != "Necromantic" || name != "Deathblock") && name != parent.Name {
			return result, fmt.Errorf("missing modifier scale")
		}
		return result, nil
	}
	if input.ModifierDice != "" && !dicePattern.MatchString(strings.ToLower(input.ModifierDice)) {
		return result, fmt.Errorf("modifierDice %q is not a canonical dN token", input.ModifierDice)
	}
	rows := append([]sourceModifier(nil), input.Modifiers...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Level < rows[j].Level })
	values := make([]float64, len(rows))
	for index, row := range rows {
		if row.Level != parent.MinimumLevel+index {
			return result, fmt.Errorf("modifier levels must cover every item level from %d through %d", parent.MinimumLevel, maximumItemLevel)
		}
		value, err := strconv.ParseFloat(row.Value.String(), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return result, fmt.Errorf("modifier level %d has invalid finite numeric value %q", row.Level, row.Value)
		}
		if input.ModifierDice != "" && (math.Trunc(value) != value || value <= 0) {
			return result, fmt.Errorf("dice modifier level %d must be a positive integer", row.Level)
		}
		if _, isPercent := percentEffects[name]; isPercent {
			value = value * 100
		}
		values[index] = value
	}
	if rows[len(rows)-1].Level != maximumItemLevel {
		return result, fmt.Errorf("modifier scale does not reach item level %d", maximumItemLevel)
	}
	unit := "number"
	if input.ModifierDice != "" {
		unit = "dice"
	} else if _, isPercent := percentEffects[name]; isPercent {
		unit = "percent"
	}
	result.Modifier = &contracts.EssenceCraftingModifier{Kind: "by-item-level", Unit: unit, Die: strings.ToLower(input.ModifierDice), Bands: coalesceBands(rows, values)}
	return result, nil
}

func coalesceBands(rows []sourceModifier, values []float64) []contracts.EssenceCraftingModifierBand {
	result := make([]contracts.EssenceCraftingModifierBand, 0, len(rows))
	for index, row := range rows {
		if len(result) != 0 && result[len(result)-1].Value == values[index] && result[len(result)-1].MaximumItemLevel+1 == row.Level {
			result[len(result)-1].MaximumItemLevel = row.Level
			continue
		}
		result = append(result, contracts.EssenceCraftingModifierBand{MinimumItemLevel: row.Level, MaximumItemLevel: row.Level, Value: values[index]})
	}
	return result
}

func transformSourceRecipe(input *sourceRecipe, binding string, ingredientNames map[string]struct{}, recipeOwners map[int]string, owner string) (contracts.EssenceCraftingRecipe, error) {
	if input == nil {
		return contracts.EssenceCraftingRecipe{}, fmt.Errorf("%s %s recipe is required", owner, binding)
	}
	if input.RecipeID <= 0 {
		return contracts.EssenceCraftingRecipe{}, fmt.Errorf("%s %s recipe has invalid recipeId", owner, binding)
	}
	if previous, exists := recipeOwners[input.RecipeID]; exists {
		return contracts.EssenceCraftingRecipe{}, fmt.Errorf("recipe source ID %d is shared by %s and %s", input.RecipeID, previous, owner)
	}
	recipeOwners[input.RecipeID] = owner
	if input.Level <= 0 || input.Level > maximumCraftLevel {
		return contracts.EssenceCraftingRecipe{}, fmt.Errorf("%s %s recipe crafting level %d is outside 1-%d", owner, binding, input.Level, maximumCraftLevel)
	}
	if input.Essence <= 0 {
		return contracts.EssenceCraftingRecipe{}, fmt.Errorf("%s %s recipe has invalid Magic Item Essence quantity", owner, binding)
	}
	requirements := []contracts.EssenceCraftingRequirement{{Kind: "ingredient", IngredientID: opaqueID("ingredient", magicItemEssence), Quantity: input.Essence}}
	seen := map[string]struct{}{magicItemEssence: {}}
	for index, requirement := range input.Collectibles {
		name := strings.TrimSpace(requirement.Name)
		if name == "" || requirement.Quantity <= 0 {
			return contracts.EssenceCraftingRecipe{}, fmt.Errorf("%s %s recipe collectible[%d] has invalid name or quantity", owner, binding, index)
		}
		if _, exists := seen[name]; exists {
			return contracts.EssenceCraftingRecipe{}, fmt.Errorf("%s %s recipe repeats ingredient %q", owner, binding, name)
		}
		seen[name] = struct{}{}
		ingredientNames[name] = struct{}{}
		requirements = append(requirements, contracts.EssenceCraftingRequirement{Kind: "ingredient", IngredientID: opaqueID("ingredient", name), Quantity: requirement.Quantity})
	}
	sortRequirements(requirements)
	return contracts.EssenceCraftingRecipe{ID: "recipe-source-" + strconv.Itoa(input.RecipeID), Kind: "enhancement-shard", SourceRecipeID: strconv.Itoa(input.RecipeID), Binding: binding, CraftingLevel: input.Level, Requirements: requirements}, nil
}

func materializeMinimumLevelShards(ingredientNames map[string]struct{}) ([]contracts.EssenceCraftingMinimumShard, []contracts.EssenceCraftingRecipe) {
	ingredientNames[purifiedFragment] = struct{}{}
	shards := make([]contracts.EssenceCraftingMinimumShard, 0, maximumItemLevel)
	recipes := make([]contracts.EssenceCraftingRecipe, 0, maximumItemLevel*2)
	for level := minimumItemLevel; level <= maximumItemLevel; level++ {
		boundCraftLevel := level * 10
		if level == 1 {
			boundCraftLevel = 1
		}
		boundID := fmt.Sprintf("recipe-minimum-level-bound-%02d", level)
		unboundID := fmt.Sprintf("recipe-minimum-level-unbound-%02d", level)
		unboundCraftLevel := max(150, (level+5)*10)
		boundRequirements := []contracts.EssenceCraftingRequirement{{Kind: "ingredient", IngredientID: opaqueID("ingredient", magicItemEssence), Quantity: level * 10}}
		unboundRequirements := []contracts.EssenceCraftingRequirement{{Kind: "ingredient", IngredientID: opaqueID("ingredient", magicItemEssence), Quantity: unboundCraftLevel * 2}}
		if quantity := unboundPurifiedQuantity(level); quantity != 0 {
			unboundRequirements = append(unboundRequirements, contracts.EssenceCraftingRequirement{Kind: "ingredient", IngredientID: opaqueID("ingredient", purifiedFragment), Quantity: quantity})
		}
		recipes = append(recipes,
			contracts.EssenceCraftingRecipe{ID: boundID, Kind: "minimum-level-shard", ItemLevel: level, Binding: "bound", CraftingLevel: boundCraftLevel, Requirements: boundRequirements},
			contracts.EssenceCraftingRecipe{ID: unboundID, Kind: "minimum-level-shard", ItemLevel: level, Binding: "unbound", CraftingLevel: unboundCraftLevel, Requirements: unboundRequirements},
		)
		shards = append(shards, contracts.EssenceCraftingMinimumShard{ItemLevel: level, Recipes: contracts.EssenceCraftingRecipePair{BoundRecipeID: boundID, UnboundRecipeID: unboundID}})
	}
	return shards, recipes
}

func unboundPurifiedQuantity(level int) int {
	switch {
	case level >= 31:
		return 15
	case level >= 26:
		return 10
	case level >= 21:
		return 5
	default:
		return 0
	}
}

func transformAugments(ctx context.Context, master dataset.Master) ([]contracts.EssenceCraftingAugment, map[string]string, map[string]string, error) {
	result := make([]contracts.EssenceCraftingAugment, 0)
	bonusNames, effectNames := map[string]string{}, map[string]string{}
	seen := map[string]string{}
	for _, record := range master.Augments {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		typeID, relevant := normalizeAugmentType(record.Augment.AugmentType)
		if !relevant {
			continue
		}
		name := strings.TrimSpace(record.Augment.Name)
		if name == "" {
			return nil, nil, nil, fmt.Errorf("augment %s has an empty name", record.Source())
		}
		if record.Augment.MinLevel == nil || *record.Augment.MinLevel < minimumItemLevel || *record.Augment.MinLevel > maximumItemLevel {
			return nil, nil, nil, fmt.Errorf("augment %s has invalid minimum item level", record.Source())
		}
		id := opaqueID("augment", record.File+"\x00"+name)
		if previous, exists := seen[id]; exists {
			return nil, nil, nil, fmt.Errorf("augment ID %q duplicates master records %s and %s", id, previous, record.Source())
		}
		seen[id] = record.Source()
		augmentEffects := append([]dataset.PartialEnhancementOut(nil), record.Augment.EffectsAdded...)
		matchedSetMarkers := map[string]struct{}{}
		for setIndex, set := range record.Augment.SetBonus {
			setName := strings.TrimSpace(set.Name)
			if setName == "" {
				return nil, nil, nil, fmt.Errorf("augment %s setBonus[%d]: name is required", record.Source(), setIndex)
			}
			// ItemSet membership is stored canonically in SetBonus rather than
			// EffectsAdded. Project it into the Essence Crafting effect list while
			// avoiding duplication if older or externally supplied canonical data
			// already contains the textual marker.
			marker := "Item Set: " + setName
			if _, alreadyMatched := matchedSetMarkers[marker]; alreadyMatched {
				// Preserve duplicate-effect validation for malformed canonical input.
				augmentEffects = append(augmentEffects, dataset.PartialEnhancementOut{Name: marker})
				continue
			}
			found := false
			for _, effect := range augmentEffects {
				if strings.TrimSpace(effect.Name) == marker && strings.TrimSpace(effect.Bonus) == "" {
					found = true
					break
				}
			}
			if !found {
				augmentEffects = append(augmentEffects, dataset.PartialEnhancementOut{Name: marker})
			}
			matchedSetMarkers[marker] = struct{}{}
		}
		effects := make([]contracts.EssenceCraftingEffect, 0, len(augmentEffects))
		seenEffects := map[string]struct {
			index  int
			effect dataset.PartialEnhancementOut
		}{}
		for index, effect := range augmentEffects {
			transformed, err := transformAugmentEffect(effect)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("augment %s effect[%d]: %w", record.Source(), index, err)
			}
			if first, exists := seenEffects[transformed.ID]; exists {
				return nil, nil, nil, fmt.Errorf(
					"augment %s repeats semantic effect %q with %s at effects[%d] and effects[%d]; generated ID %q; first modifier=%v, duplicate modifier=%v",
					record.Source(), transformed.DisplayName, describeBonus(effect.Bonus), first.index, index, transformed.ID, first.effect.Modifier, effect.Modifier,
				)
			}
			seenEffects[transformed.ID] = struct {
				index  int
				effect dataset.PartialEnhancementOut
			}{index: index, effect: effect}
			effects = append(effects, transformed)
			if transformed.BonusTypeID != "" {
				bonusNames[transformed.BonusTypeID] = strings.TrimSpace(effect.Bonus)
			}
			effectNames[transformed.ID] = transformed.DisplayName
		}
		sortEffects(effects)
		result = append(result, contracts.EssenceCraftingAugment{ID: id, DisplayName: name, AugmentTypeID: typeID, MinimumItemLevel: *record.Augment.MinLevel, Effects: effects})
	}
	return result, bonusNames, effectNames, nil
}

func describeBonus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "empty bonus"
	}
	return fmt.Sprintf("bonus %q", strings.TrimSpace(value))
}

func normalizeAugmentType(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "red", "blue", "yellow", "green", "purple", "orange", "colorless":
		return value, true
	default:
		return "", false
	}
}

func transformAugmentEffect(input dataset.PartialEnhancementOut) (contracts.EssenceCraftingEffect, error) {
	name, bonus := strings.TrimSpace(input.Name), strings.TrimSpace(input.Bonus)
	if name == "" {
		return contracts.EssenceCraftingEffect{}, fmt.Errorf("name is required")
	}
	result := contracts.EssenceCraftingEffect{ID: opaqueID("effect", strings.ToLower(bonus)+"\x00"+name), DisplayName: name, BonusTypeID: bonusID(bonus)}
	if input.Modifier == nil {
		return result, nil
	}
	switch value := input.Modifier.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return result, fmt.Errorf("modifier is not finite")
		}
		unit := "number"
		if _, isPercent := percentEffects[name]; isPercent {
			unit, value = "percent", value*100
		}
		result.Modifier = &contracts.EssenceCraftingModifier{Kind: "fixed", Unit: unit, Value: value}
	case int:
		result.Modifier = &contracts.EssenceCraftingModifier{Kind: "fixed", Unit: "number", Value: value}
	case string:
		value = strings.TrimSpace(value)
		if value == "" {
			return result, fmt.Errorf("modifier text is blank")
		}
		if match := regexp.MustCompile(`^([1-9][0-9]*)d([1-9][0-9]*)$`).FindStringSubmatch(strings.ToLower(value)); match != nil {
			count, _ := strconv.Atoi(match[1])
			result.Modifier = &contracts.EssenceCraftingModifier{Kind: "fixed", Unit: "dice", Value: count, Die: "d" + match[2]}
		} else if _, isPercent := percentEffects[name]; isPercent {
			percent, err := parsePercentModifier(value)
			if err == nil {
				result.Modifier = &contracts.EssenceCraftingModifier{Kind: "fixed", Unit: "percent", Value: percent}
			} else {
				result.Modifier = &contracts.EssenceCraftingModifier{Kind: "fixed", Unit: "text", Value: value}
			}
		} else {
			result.Modifier = &contracts.EssenceCraftingModifier{Kind: "fixed", Unit: "text", Value: value}
		}
	default:
		return result, fmt.Errorf("unsupported modifier type %T", input.Modifier)
	}
	return result, nil
}

func parsePercentModifier(value string) (float64, error) {
	if strings.HasSuffix(value, "%") {
		return strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")), 64)
	}
	decimal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return decimal * 100, nil
}

func ingredients(names map[string]struct{}) []contracts.EssenceCraftingIngredient {
	result := make([]contracts.EssenceCraftingIngredient, 0, len(names))
	for name := range names {
		result = append(result, contracts.EssenceCraftingIngredient{ID: opaqueID("ingredient", name), DisplayName: name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func namedRecords(names map[string]string) []contracts.EssenceCraftingNamedRecord {
	result := make([]contracts.EssenceCraftingNamedRecord, 0, len(names))
	for id, name := range names {
		result = append(result, contracts.EssenceCraftingNamedRecord{ID: id, DisplayName: name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func sortEffects(values []contracts.EssenceCraftingEffect) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].DisplayName == values[j].DisplayName {
			return values[i].ID < values[j].ID
		}
		return values[i].DisplayName < values[j].DisplayName
	})
}
func sortRequirements(values []contracts.EssenceCraftingRequirement) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Kind != values[j].Kind {
			return values[i].Kind == "ingredient"
		}
		if values[i].Kind == "ingredient" {
			return values[i].IngredientID < values[j].IngredientID
		}
		return values[i].RecipeID < values[j].RecipeID
	})
}

func sortDomain(result *contracts.EssenceCraftingDomain) {
	sort.Slice(result.Enhancements, func(i, j int) bool { return result.Enhancements[i].ID < result.Enhancements[j].ID })
	sort.Slice(result.Recipes, func(i, j int) bool { return result.Recipes[i].ID < result.Recipes[j].ID })
	sort.Slice(result.MinimumLevelShards, func(i, j int) bool {
		return result.MinimumLevelShards[i].ItemLevel < result.MinimumLevelShards[j].ItemLevel
	})
	sort.Slice(result.Augments, func(i, j int) bool { return result.Augments[i].ID < result.Augments[j].ID })
}
