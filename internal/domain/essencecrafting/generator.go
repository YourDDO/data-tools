// Package essencecrafting generates the normalized Essence Crafting domain.
package essencecrafting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
)

const Name = "essence-crafting"

const (
	minimumItemLevel  = 1
	maximumItemLevel  = 36
	maximumCraftLevel = 500
	magicItemEssence  = "Magic Item Essence"
	purifiedFragment  = "Purified Eberron Dragonshard Fragment"
	markOfCannith     = "Mark of House Cannith"
)

type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

func (g Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	return g.GenerateWithManual(ctx, master, filepath.Join(outputRoot, "manual"), outputRoot)
}

func (Generator) GenerateWithManual(ctx context.Context, master dataset.Master, manualRoot, outputRoot string) (domain.Result, error) {
	source, err := loadSource(manualRoot)
	if err != nil {
		return domain.Result{}, err
	}
	payload, err := build(ctx, master, source)
	if err != nil {
		return domain.Result{}, err
	}
	file, err := domain.WriteJSON(outputRoot, Name, "essence-crafting.json", payload)
	if err != nil {
		return domain.Result{}, fmt.Errorf("write Essence Crafting domain: %w", err)
	}
	return domain.Result{Domain: Name, Files: []contracts.GeneratedFileMetadata{file}}, nil
}

func build(ctx context.Context, master dataset.Master, source []sourceEnhancement) (contracts.EssenceCraftingDomain, error) {
	result := contracts.EssenceCraftingDomain{
		SchemaVersion:  contracts.EssenceCraftingSchemaVersion,
		Rules:          rules(),
		ItemCategories: itemCategories(),
		AugmentTypes:   augmentTypes(),
	}
	ingredientNames := map[string]struct{}{magicItemEssence: {}, markOfCannith: {}}
	bonusNames := map[string]string{}
	recipeOwners := map[int]string{}
	enhancementIDs := map[string]string{}

	for index, input := range source {
		if err := ctx.Err(); err != nil {
			return contracts.EssenceCraftingDomain{}, err
		}
		context := fmt.Sprintf("enhancement[%d]", index)
		if err := validateSourceEnhancement(context, input); err != nil {
			return contracts.EssenceCraftingDomain{}, err
		}
		// Some canonical rows define a nonnumeric effect directly by the shard
		// name. Preserve that exact source label as one effect rather than
		// dropping a valid selectable recipe.
		if len(input.Effects) == 0 {
			input.Effects = []sourceEffect{{Name: input.Name}}
		}
		id := opaqueID("enhancement", strconv.Itoa(input.Bound.RecipeID)+":"+strconv.Itoa(input.Unbound.RecipeID))
		if previous, exists := enhancementIDs[id]; exists {
			return contracts.EssenceCraftingDomain{}, fmt.Errorf("%s: enhancement ID %q duplicates %s", context, id, previous)
		}
		enhancementIDs[id] = context

		placements, err := normalizePlacements(input)
		if err != nil {
			return contracts.EssenceCraftingDomain{}, fmt.Errorf("%s: %w", context, err)
		}
		effects := make([]contracts.EssenceCraftingEffect, 0, len(input.Effects))
		seenEffectKeys := map[string]struct{}{}
		for effectIndex, inputEffect := range input.Effects {
			effect, err := transformScaledEffect(id, input, inputEffect)
			if err != nil {
				return contracts.EssenceCraftingDomain{}, fmt.Errorf("%s effect[%d]: %w", context, effectIndex, err)
			}
			semanticKey := effectSemanticKey(inputEffect.Bonus, inputEffect.Name)
			if _, exists := seenEffectKeys[semanticKey]; exists {
				return contracts.EssenceCraftingDomain{}, fmt.Errorf("%s: repeats semantic effect %q with %s", context, effect.DisplayName, describeBonus(inputEffect.Bonus))
			}
			seenEffectKeys[semanticKey] = struct{}{}
			if effect.BonusTypeID != "" {
				bonusNames[effect.BonusTypeID] = strings.TrimSpace(inputEffect.Bonus)
			}
			effects = append(effects, effect)
		}
		sortEffects(effects)

		bound, err := transformSourceRecipe(input.Bound, "bound", ingredientNames, recipeOwners, context)
		if err != nil {
			return contracts.EssenceCraftingDomain{}, err
		}
		unbound, err := transformSourceRecipe(input.Unbound, "unbound", ingredientNames, recipeOwners, context)
		if err != nil {
			return contracts.EssenceCraftingDomain{}, err
		}
		result.Recipes = append(result.Recipes, bound, unbound)
		result.Enhancements = append(result.Enhancements, contracts.EssenceCraftingEnhancement{
			ID: id, DisplayName: strings.TrimSpace(input.Name), MinimumItemLevel: input.MinimumLevel,
			Placements: placements, Effects: effects,
			Recipes: contracts.EssenceCraftingRecipePair{BoundRecipeID: bound.ID, UnboundRecipeID: unbound.ID},
		})
	}

	minimumShards, minimumRecipes := materializeMinimumLevelShards(ingredientNames)
	result.MinimumLevelShards = minimumShards
	result.Recipes = append(result.Recipes, minimumRecipes...)
	augments, augmentBonusNames, err := transformAugments(ctx, master)
	if err != nil {
		return contracts.EssenceCraftingDomain{}, err
	}
	for id, displayName := range augmentBonusNames {
		bonusNames[id] = displayName
	}
	result.Augments = augments
	result.BonusTypes = namedRecords(bonusNames)
	result.Ingredients = ingredients(ingredientNames)
	sortDomain(&result)
	if err := validateDomain(result); err != nil {
		return contracts.EssenceCraftingDomain{}, err
	}
	return result, nil
}

func rules() contracts.EssenceCraftingRules {
	colors := []string{"blue", "colorless", "green", "orange", "purple", "red", "yellow"}
	slots := make([]contracts.EssenceCraftingAugmentSlotType, 0, len(colors))
	for _, color := range colors {
		accepted := make([]string, 0, len(colors))
		for _, augmentType := range colors {
			if domain.AugmentTypesCompatible(color, augmentType) {
				accepted = append(accepted, augmentType)
			}
		}
		slots = append(slots, contracts.EssenceCraftingAugmentSlotType{ID: color, DisplayName: title(color), AcceptsAugmentTypeIDs: accepted})
	}
	return contracts.EssenceCraftingRules{
		SupportedItemLevels:  contracts.EssenceCraftingLevelRange{Minimum: minimumItemLevel, Maximum: maximumItemLevel},
		MaximumCraftingLevel: maximumCraftLevel,
		ExtraAffix: contracts.EssenceCraftingExtraAffix{
			Position: "extra", ConsumedWhen: "extra-enhancement-applied",
			MarkRequirement: contracts.EssenceCraftingRequirement{Kind: "ingredient", IngredientID: opaqueID("ingredient", markOfCannith), Quantity: 1},
		},
		AugmentSlotTypes: slots,
		// Item-category slot placement rules remain empty until their gameplay
		// policy is approved. The contract deliberately does not import the old
		// frontend table as a surrogate source of truth.
		AugmentSlotPlacements: []contracts.EssenceCraftingAugmentPlacement{},
	}
}

func itemCategories() []contracts.EssenceCraftingNamedRecord {
	values := []struct{ id, name string }{
		{"armor", "Armor"}, {"belt", "Belt"}, {"boots", "Boots"}, {"bracers", "Bracers"}, {"cloak", "Cloak"},
		{"gloves", "Gloves"}, {"goggles", "Goggles"}, {"head", "Head"}, {"necklace", "Necklace"}, {"orb", "Orb"},
		{"ring", "Ring"}, {"rune-arm", "Rune Arm"}, {"shield", "Shield"}, {"trinket", "Trinket"}, {"weapon", "Weapon"},
	}
	result := make([]contracts.EssenceCraftingNamedRecord, len(values))
	for i, value := range values {
		result[i] = contracts.EssenceCraftingNamedRecord{ID: value.id, DisplayName: value.name}
	}
	return result
}

func augmentTypes() []contracts.EssenceCraftingNamedRecord {
	values := []string{"blue", "colorless", "green", "orange", "purple", "red", "yellow"}
	result := make([]contracts.EssenceCraftingNamedRecord, len(values))
	for i, value := range values {
		result[i] = contracts.EssenceCraftingNamedRecord{ID: value, DisplayName: title(value)}
	}
	return result
}

func title(value string) string {
	if value == "colorless" {
		return "Colorless"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func opaqueID(namespace, source string) string {
	sum := sha256.Sum256([]byte(namespace + "\x00" + source))
	return namespace + "-" + hex.EncodeToString(sum[:8])
}

func bonusID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return opaqueID("bonus", strings.ToLower(value))
}

// effectSemanticKey preserves the original semantic identity for detecting
// duplicate effects within one parent. It deliberately excludes the parent ID.
func effectSemanticKey(bonus, displayName string) string {
	return strings.ToLower(strings.TrimSpace(bonus)) + "\x00" + strings.TrimSpace(displayName)
}

func effectID(parentID, bonus, displayName string) string {
	return opaqueID("effect", parentID+"\x00"+bonusID(bonus)+"\x00"+strings.TrimSpace(displayName))
}
