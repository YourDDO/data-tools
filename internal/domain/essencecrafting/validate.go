package essencecrafting

import (
	"fmt"
	"regexp"
	"sort"

	"yourddo-data-tools/internal/contracts"
)

func validateDomain(value contracts.EssenceCraftingDomain) error {
	if value.SchemaVersion != contracts.EssenceCraftingSchemaVersion {
		return fmt.Errorf("schemaVersion must be %d", contracts.EssenceCraftingSchemaVersion)
	}
	if value.Rules.SupportedItemLevels.Minimum != minimumItemLevel || value.Rules.SupportedItemLevels.Maximum != maximumItemLevel || value.Rules.MaximumCraftingLevel != maximumCraftLevel {
		return fmt.Errorf("supported levels and crafting cap do not match approved policy")
	}
	if err := uniqueNamed("item category", value.ItemCategories); err != nil {
		return err
	}
	if err := uniqueNamed("bonus type", value.BonusTypes); err != nil {
		return err
	}
	if err := uniqueNamed("augment type", value.AugmentTypes); err != nil {
		return err
	}
	categories, augmentTypes := namedSet(value.ItemCategories), namedSet(value.AugmentTypes)
	ingredients, recipes := map[string]struct{}{}, map[string]contracts.EssenceCraftingRecipe{}
	for _, ingredient := range value.Ingredients {
		if ingredient.ID == "" || ingredient.DisplayName == "" {
			return fmt.Errorf("ingredient has blank ID or display name")
		}
		if _, exists := ingredients[ingredient.ID]; exists {
			return fmt.Errorf("duplicate ingredient ID %q", ingredient.ID)
		}
		ingredients[ingredient.ID] = struct{}{}
	}
	for _, recipe := range value.Recipes {
		if _, exists := recipes[recipe.ID]; exists {
			return fmt.Errorf("duplicate recipe ID %q", recipe.ID)
		}
		if recipe.ID == "" || (recipe.Binding != "bound" && recipe.Binding != "unbound") || recipe.CraftingLevel <= 0 || recipe.CraftingLevel > maximumCraftLevel {
			return fmt.Errorf("recipe %q has invalid identity, binding, or crafting level", recipe.ID)
		}
		if recipe.Kind != "enhancement-shard" && recipe.Kind != "minimum-level-shard" {
			return fmt.Errorf("recipe %q has unsupported kind %q", recipe.ID, recipe.Kind)
		}
		if recipe.Kind == "enhancement-shard" && recipe.SourceRecipeID == "" {
			return fmt.Errorf("enhancement recipe %q has no source recipe ID", recipe.ID)
		}
		if recipe.Kind == "minimum-level-shard" && (recipe.ItemLevel < minimumItemLevel || recipe.ItemLevel > maximumItemLevel) {
			return fmt.Errorf("minimum-level recipe %q has invalid item level", recipe.ID)
		}
		seenRequirements, essenceCount := map[string]struct{}{}, 0
		for _, requirement := range recipe.Requirements {
			if requirement.Quantity <= 0 {
				return fmt.Errorf("recipe %q has invalid requirement quantity", recipe.ID)
			}
			key := requirement.Kind + "\x00" + requirement.IngredientID + "\x00" + requirement.RecipeID
			if _, exists := seenRequirements[key]; exists {
				return fmt.Errorf("recipe %q repeats requirement", recipe.ID)
			}
			seenRequirements[key] = struct{}{}
			switch requirement.Kind {
			case "ingredient":
				if _, exists := ingredients[requirement.IngredientID]; !exists {
					return fmt.Errorf("recipe %q references missing ingredient %q", recipe.ID, requirement.IngredientID)
				}
				if requirement.IngredientID == opaqueID("ingredient", magicItemEssence) {
					essenceCount++
				}
			case "recipe":
				if requirement.RecipeID == "" {
					return fmt.Errorf("recipe %q has empty nested recipe reference", recipe.ID)
				}
			default:
				return fmt.Errorf("recipe %q has unknown requirement kind %q", recipe.ID, requirement.Kind)
			}
		}
		if essenceCount != 1 {
			return fmt.Errorf("recipe %q has %d Magic Item Essence requirements, want one", recipe.ID, essenceCount)
		}
		recipes[recipe.ID] = recipe
	}
	if err := validateRecipeCycles(recipes); err != nil {
		return err
	}
	referencedRecipes := map[string]int{}
	seenEnhancements := map[string]struct{}{}
	seenEffectIDs := map[string]string{}
	for _, enhancement := range value.Enhancements {
		if enhancement.ID == "" || enhancement.DisplayName == "" || enhancement.MinimumItemLevel < minimumItemLevel || enhancement.MinimumItemLevel > maximumItemLevel {
			return fmt.Errorf("enhancement has invalid identity or minimum item level")
		}
		if _, exists := seenEnhancements[enhancement.ID]; exists {
			return fmt.Errorf("duplicate enhancement ID %q", enhancement.ID)
		}
		seenEnhancements[enhancement.ID] = struct{}{}
		if len(enhancement.Placements) == 0 || len(enhancement.Effects) == 0 {
			return fmt.Errorf("enhancement %q has no placements or effects", enhancement.ID)
		}
		seenPlacements := map[string]struct{}{}
		for _, placement := range enhancement.Placements {
			if placement.Position != "prefix" && placement.Position != "suffix" && placement.Position != "extra" {
				return fmt.Errorf("enhancement %q has unsupported position %q", enhancement.ID, placement.Position)
			}
			if len(placement.ItemCategoryIDs) == 0 {
				return fmt.Errorf("enhancement %q has empty placement", enhancement.ID)
			}
			for _, category := range placement.ItemCategoryIDs {
				if _, exists := categories[category]; !exists {
					return fmt.Errorf("enhancement %q has unsupported item category %q", enhancement.ID, category)
				}
				key := placement.Position + "\x00" + category
				if _, exists := seenPlacements[key]; exists {
					return fmt.Errorf("enhancement %q repeats placement %s", enhancement.ID, key)
				}
				seenPlacements[key] = struct{}{}
			}
		}
		seenEffects := map[string]struct{}{}
		for _, effect := range enhancement.Effects {
			if err := validateEffect(enhancement.ID, enhancement.MinimumItemLevel, effect, value.BonusTypes); err != nil {
				return err
			}
			semanticKey := effect.BonusTypeID + "\x00" + effect.DisplayName
			if _, exists := seenEffects[semanticKey]; exists {
				return fmt.Errorf("enhancement %q repeats semantic effect %q", enhancement.ID, effect.DisplayName)
			}
			seenEffects[semanticKey] = struct{}{}
			if previous, exists := seenEffectIDs[effect.ID]; exists {
				return fmt.Errorf("duplicate effect ID %q belongs to %s and enhancement %q", effect.ID, previous, enhancement.ID)
			}
			seenEffectIDs[effect.ID] = fmt.Sprintf("enhancement %q", enhancement.ID)
		}
		for _, pair := range []struct{ id, binding string }{{enhancement.Recipes.BoundRecipeID, "bound"}, {enhancement.Recipes.UnboundRecipeID, "unbound"}} {
			recipe, exists := recipes[pair.id]
			if !exists || recipe.Kind != "enhancement-shard" || recipe.Binding != pair.binding {
				return fmt.Errorf("enhancement %q has missing %s recipe reference", enhancement.ID, pair.binding)
			}
			referencedRecipes[pair.id]++
		}
	}
	if len(value.MinimumLevelShards) != maximumItemLevel {
		return fmt.Errorf("minimum-level shard coverage has %d rows, want %d", len(value.MinimumLevelShards), maximumItemLevel)
	}
	for index, shard := range value.MinimumLevelShards {
		level := index + minimumItemLevel
		if shard.ItemLevel != level {
			return fmt.Errorf("minimum-level shards are missing item level %d", level)
		}
		for _, pair := range []struct{ id, binding string }{{shard.Recipes.BoundRecipeID, "bound"}, {shard.Recipes.UnboundRecipeID, "unbound"}} {
			recipe, exists := recipes[pair.id]
			if !exists || recipe.Kind != "minimum-level-shard" || recipe.Binding != pair.binding || recipe.ItemLevel != level {
				return fmt.Errorf("minimum-level shard %d has missing %s recipe", level, pair.binding)
			}
			referencedRecipes[pair.id]++
		}
	}
	for id := range recipes {
		if referencedRecipes[id] != 1 {
			return fmt.Errorf("recipe %q is orphaned or has %d owners", id, referencedRecipes[id])
		}
	}
	for _, augment := range value.Augments {
		if _, exists := augmentTypes[augment.AugmentTypeID]; !exists {
			return fmt.Errorf("augment %q has unknown augment type %q", augment.ID, augment.AugmentTypeID)
		}
		if augment.MinimumItemLevel < minimumItemLevel || augment.MinimumItemLevel > maximumItemLevel {
			return fmt.Errorf("augment %q has invalid minimum item level", augment.ID)
		}
		seenEffects := map[string]struct{}{}
		for _, effect := range augment.Effects {
			if err := validateEffect(augment.ID, augment.MinimumItemLevel, effect, value.BonusTypes); err != nil {
				return err
			}
			semanticKey := effect.BonusTypeID + "\x00" + effect.DisplayName
			if _, exists := seenEffects[semanticKey]; exists {
				return fmt.Errorf("augment %q repeats semantic effect %q", augment.ID, effect.DisplayName)
			}
			seenEffects[semanticKey] = struct{}{}
			if previous, exists := seenEffectIDs[effect.ID]; exists {
				return fmt.Errorf("duplicate effect ID %q belongs to %s and augment %q", effect.ID, previous, augment.ID)
			}
			seenEffectIDs[effect.ID] = fmt.Sprintf("augment %q", augment.ID)
		}
	}
	if _, exists := ingredients[value.Rules.ExtraAffix.MarkRequirement.IngredientID]; !exists {
		return fmt.Errorf("extra-affix Mark references a missing ingredient")
	}
	return nil
}

func uniqueNamed(kind string, values []contracts.EssenceCraftingNamedRecord) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value.ID == "" || value.DisplayName == "" {
			return fmt.Errorf("%s has blank ID or display name", kind)
		}
		if _, exists := seen[value.ID]; exists {
			return fmt.Errorf("duplicate %s ID %q", kind, value.ID)
		}
		seen[value.ID] = struct{}{}
	}
	return nil
}
func namedSet(values []contracts.EssenceCraftingNamedRecord) map[string]struct{} {
	result := map[string]struct{}{}
	for _, value := range values {
		result[value.ID] = struct{}{}
	}
	return result
}

func validateEffect(owner string, minimum int, effect contracts.EssenceCraftingEffect, bonusTypes []contracts.EssenceCraftingNamedRecord) error {
	if effect.ID == "" || effect.DisplayName == "" {
		return fmt.Errorf("%s has an effect with blank ID or display name", owner)
	}
	if effect.BonusTypeID != "" {
		if _, exists := namedSet(bonusTypes)[effect.BonusTypeID]; !exists {
			return fmt.Errorf("%s effect %q references missing bonus type", owner, effect.ID)
		}
	}
	if effect.Modifier == nil {
		return nil
	}
	modifier := effect.Modifier
	if modifier.Kind == "fixed" {
		if modifier.Unit != "number" && modifier.Unit != "percent" && modifier.Unit != "dice" && modifier.Unit != "text" {
			return fmt.Errorf("%s effect %q has invalid modifier unit", owner, effect.ID)
		}
		if modifier.Unit == "dice" && !regexp.MustCompile(`^d[1-9][0-9]*$`).MatchString(modifier.Die) {
			return fmt.Errorf("%s effect %q has invalid die", owner, effect.ID)
		}
		return nil
	}
	if modifier.Kind != "by-item-level" || modifier.Unit == "text" || len(modifier.Bands) == 0 {
		return fmt.Errorf("%s effect %q has invalid scaled modifier", owner, effect.ID)
	}
	next := minimum
	for _, band := range modifier.Bands {
		if band.MinimumItemLevel != next || band.MaximumItemLevel < band.MinimumItemLevel || band.MaximumItemLevel > maximumItemLevel {
			return fmt.Errorf("%s effect %q has invalid modifier bands", owner, effect.ID)
		}
		next = band.MaximumItemLevel + 1
	}
	if next != maximumItemLevel+1 {
		return fmt.Errorf("%s effect %q modifier bands do not cover through %d", owner, effect.ID, maximumItemLevel)
	}
	return nil
}

func validateRecipeCycles(recipes map[string]contracts.EssenceCraftingRecipe) error {
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("nested recipe cycle includes %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		recipe := recipes[id]
		for _, requirement := range recipe.Requirements {
			if requirement.Kind == "recipe" {
				if _, exists := recipes[requirement.RecipeID]; !exists {
					return fmt.Errorf("recipe %q references missing recipe %q", id, requirement.RecipeID)
				}
				if err := visit(requirement.RecipeID); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	ids := make([]string, 0, len(recipes))
	for id := range recipes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
