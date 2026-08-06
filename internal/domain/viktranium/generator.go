// Package viktranium builds the fully materialized Viktranium planning domain.
package viktranium

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain"
)

const Name = "viktranium"

type Generator struct{}

func New() Generator           { return Generator{} }
func (Generator) Name() string { return Name }

func (g Generator) Generate(ctx context.Context, master dataset.Master, outputRoot string) (domain.Result, error) {
	return g.GenerateWithManual(ctx, master, filepath.Join(outputRoot, "manual"), outputRoot)
}

func (Generator) GenerateWithManual(ctx context.Context, master dataset.Master, manualRoot, outputRoot string) (domain.Result, error) {
	source, err := loadSources(manualRoot)
	if err != nil {
		return domain.Result{}, err
	}
	payload, err := build(ctx, master, source)
	if err != nil {
		return domain.Result{}, err
	}
	file, err := domain.WriteJSON(outputRoot, Name, "viktranium.json", payload)
	if err != nil {
		return domain.Result{}, fmt.Errorf("write Viktranium dataset: %w", err)
	}
	return domain.Result{Domain: Name, Files: []contracts.GeneratedFileMetadata{file}}, nil
}

type recipeTarget struct {
	kind  string
	index int
}

func build(ctx context.Context, master dataset.Master, source sources) (contracts.ViktraniumDataset, error) {
	// Precedence is deliberate: manual records own recipes and relationships;
	// canonical master records own item/slot and augment metadata; the canonical
	// ingredient payload owns display metadata for referenced recipe leaves.
	result := contracts.ViktraniumDataset{SchemaVersion: contracts.ViktraniumSchemaVersion}
	manualProducts := make(map[string]struct{}, len(source.Recipes))
	for _, recipe := range source.Recipes {
		manualProducts[strings.TrimSpace(recipe.Produces)] = struct{}{}
	}
	itemNames := make(map[string][]int)
	itemIDs := make(map[string]string)
	for _, record := range master.Items {
		if err := ctx.Err(); err != nil {
			return contracts.ViktraniumDataset{}, err
		}
		_, explicitlyReferenced := manualProducts[strings.TrimSpace(record.Item.Name)]
		if !eligibleItem(record.Item) && !explicitlyReferenced {
			continue
		}
		item, err := transformItem(record)
		if err != nil {
			return contracts.ViktraniumDataset{}, fmt.Errorf("item %s: %w", record.Source(), err)
		}
		identity := strings.TrimSpace(item.PageTitle)
		if previous, exists := itemIDs[item.ID]; exists {
			return contracts.ViktraniumDataset{}, fmt.Errorf("item identity collision %q between %q and %q", item.ID, previous, identity)
		}
		itemIDs[item.ID] = identity
		result.Items = append(result.Items, item)
		itemNames[strings.TrimSpace(item.Name)] = append(itemNames[strings.TrimSpace(item.Name)], len(result.Items)-1)
	}
	if len(result.Items) == 0 {
		return contracts.ViktraniumDataset{}, fmt.Errorf("generated item collection is empty")
	}

	augmentNames := make(map[string][]int)
	augmentIDs := make(map[string]string)
	for _, record := range master.Augments {
		_, explicitlyReferenced := manualProducts[strings.TrimSpace(record.Augment.Name)]
		slotCompatible := false
		for _, item := range result.Items {
			for _, slot := range item.Slots {
				if compatible(slot.AugmentType, record.Augment.AugmentType) {
					slotCompatible = true
					break
				}
			}
			if slotCompatible {
				break
			}
		}
		if !explicitlyReferenced && !slotCompatible {
			continue
		}
		augment, err := transformAugment(record)
		if err != nil {
			return contracts.ViktraniumDataset{}, fmt.Errorf("augment %s: %w", record.Source(), err)
		}
		identity := augmentIdentity(record.Augment)
		if previous, exists := augmentIDs[augment.ID]; exists {
			return contracts.ViktraniumDataset{}, fmt.Errorf("augment identity collision %q between %q and %q", augment.ID, previous, identity)
		}
		augmentIDs[augment.ID] = identity
		result.Augments = append(result.Augments, augment)
		augmentNames[strings.TrimSpace(augment.Name)] = append(augmentNames[strings.TrimSpace(augment.Name)], len(result.Augments)-1)
	}
	for itemIndex := range result.Items {
		for slotIndex := range result.Items[itemIndex].Slots {
			slot := &result.Items[itemIndex].Slots[slotIndex]
			if !strings.HasPrefix(slot.ExistingAugmentID, "name:") {
				continue
			}
			name := strings.TrimPrefix(slot.ExistingAugmentID, "name:")
			matches := make([]int, 0)
			for _, index := range augmentNames[name] {
				if compatible(slot.AugmentType, result.Augments[index].AugmentType) {
					matches = append(matches, index)
				}
			}
			if len(matches) != 1 {
				return contracts.ViktraniumDataset{}, fmt.Errorf("item %q slot %q filled augment %q has %d compatible joins", result.Items[itemIndex].Name, slot.ID, name, len(matches))
			}
			slot.ExistingAugmentID = result.Augments[matches[0]].ID
		}
	}

	targets := make(map[int64]recipeTarget, len(source.Recipes))
	productRecipes := make(map[string][]int64)
	for _, recipe := range source.Recipes {
		name := strings.TrimSpace(recipe.Produces)
		productRecipes[name] = append(productRecipes[name], recipe.RecipeID)
		items, augments := itemNames[name], augmentNames[name]
		if len(items)+len(augments) > 1 && strings.TrimSpace(recipe.ProductEffect) != "" {
			effect := strings.TrimSpace(recipe.ProductEffect)
			effectItems := filterIndexes(items, func(index int) bool {
				return strings.TrimSpace(result.Items[index].Description) == effect
			})
			effectAugments := filterIndexes(augments, func(index int) bool {
				return strings.TrimSpace(result.Augments[index].Description) == effect
			})
			if len(effectItems)+len(effectAugments) != 0 {
				items, augments = effectItems, effectAugments
			}
		}
		if len(items)+len(augments) > 1 {
			items, augments = filterByRecipeTier(recipe.Device, items, augments, result)
		}
		if len(items)+len(augments) != 1 {
			return contracts.ViktraniumDataset{}, fmt.Errorf("recipe %d product %q has ambiguous join: %d items, %d augments", recipe.RecipeID, name, len(items), len(augments))
		}
		if len(items) == 1 {
			targets[recipe.RecipeID] = recipeTarget{kind: "item", index: items[0]}
		} else {
			targets[recipe.RecipeID] = recipeTarget{kind: "augment", index: augments[0]}
		}
	}

	ingredientByName := make(map[string][]sourceIngredient)
	for _, ingredient := range source.Ingredients {
		name := strings.TrimSpace(ingredient.Name)
		if name != "" {
			ingredientByName[name] = append(ingredientByName[name], ingredient)
		}
	}
	ingredientIDs := make(map[int64]string)
	ingredientOutput := make(map[int64]contracts.ViktraniumIngredient)
	graph := make(map[int64][]int64)
	for _, recipe := range source.Recipes {
		built := contracts.ViktraniumRecipe{
			ID: recipeID(recipe.RecipeID), DeviceID: strconv.FormatInt(recipe.DeviceID, 10),
			Device: strings.TrimSpace(recipe.Device), ProductEffect: strings.TrimSpace(recipe.ProductEffect),
		}
		for _, requirement := range recipe.Ingredients {
			if err := validateRequirement(requirement); err != nil {
				return contracts.ViktraniumDataset{}, fmt.Errorf("recipe %d requirement %q: %w", recipe.RecipeID, requirement.Name, err)
			}
			name := strings.TrimSpace(requirement.Name)
			out := contracts.ViktraniumRequirement{Name: name, Quantity: int(*requirement.Quantity)}
			nested := productRecipes[name]
			canonical := ingredientByName[name]
			switch {
			case len(nested) > 1:
				return contracts.ViktraniumDataset{}, fmt.Errorf("recipe %d requirement %q has ambiguous nested recipe join", recipe.RecipeID, name)
			case len(nested) == 1 && len(canonical) != 0:
				return contracts.ViktraniumDataset{}, fmt.Errorf("recipe %d requirement %q is both a recipe and canonical ingredient", recipe.RecipeID, name)
			case len(nested) == 1:
				out.RecipeID = recipeID(nested[0])
				graph[recipe.RecipeID] = append(graph[recipe.RecipeID], nested[0])
			case len(canonical) != 1:
				return contracts.ViktraniumDataset{}, fmt.Errorf("recipe %d ingredient %q has %d canonical joins", recipe.RecipeID, name, len(canonical))
			default:
				if previous, exists := ingredientIDs[requirement.IngredientID]; exists && previous != name {
					return contracts.ViktraniumDataset{}, fmt.Errorf("ingredient identity %d conflicts between %q and %q", requirement.IngredientID, previous, name)
				}
				ingredientIDs[requirement.IngredientID] = name
				out.IngredientID = ingredientID(requirement.IngredientID)
				ingredient := canonical[0]
				ingredientOutput[requirement.IngredientID] = contracts.ViktraniumIngredient{
					ID: out.IngredientID, Name: name, IngredientType: first(ingredient.IngredientType, ingredient.Type),
					Description: ingredient.Description, FoundIn: sortedCopy(ingredient.FoundIn),
					Icon: ingredient.Icon, Image: ingredient.Image,
				}
			}
			built.Requirements = append(built.Requirements, out)
		}
		target := targets[recipe.RecipeID]
		if target.kind == "item" {
			result.Items[target.index].Recipes = append(result.Items[target.index].Recipes, built)
		} else {
			result.Augments[target.index].Recipes = append(result.Augments[target.index].Recipes, built)
		}
	}
	if err := validateAcyclic(graph); err != nil {
		return contracts.ViktraniumDataset{}, err
	}

	selectedAugments := make([]contracts.ViktraniumAugment, 0)
	for _, augment := range result.Augments {
		relevant := len(augment.Recipes) != 0
		for _, item := range result.Items {
			for _, slot := range item.Slots {
				if compatible(slot.AugmentType, augment.AugmentType) {
					relevant = true
				}
			}
		}
		if relevant {
			selectedAugments = append(selectedAugments, augment)
		}
	}
	result.Augments = selectedAugments
	for itemIndex := range result.Items {
		for slotIndex := range result.Items[itemIndex].Slots {
			slot := &result.Items[itemIndex].Slots[slotIndex]
			matches := 0
			for _, augment := range result.Augments {
				if compatible(slot.AugmentType, augment.AugmentType) {
					matches++
				}
			}
			if matches == 0 {
				return contracts.ViktraniumDataset{}, fmt.Errorf("item %q slot %q type %q has no compatible augment", result.Items[itemIndex].Name, slot.ID, slot.AugmentType)
			}
		}
	}
	for _, ingredient := range ingredientOutput {
		result.Ingredients = append(result.Ingredients, ingredient)
	}
	sortOutput(&result)
	return result, nil
}

func filterIndexes(indexes []int, keep func(int) bool) []int {
	result := make([]int, 0, len(indexes))
	for _, index := range indexes {
		if keep(index) {
			result = append(result, index)
		}
	}
	return result
}

// Heroic and Legendary are explicit crafting-device tiers in the authoritative
// recipes. They disambiguate same-name canonical variants by minimum level;
// a tie remains an error in the normal join validation below.
func filterByRecipeTier(device string, items, augments []int, result contracts.ViktraniumDataset) ([]int, []int) {
	heroic := strings.HasPrefix(strings.TrimSpace(device), "Heroic ")
	legendary := strings.HasPrefix(strings.TrimSpace(device), "Legendary ") || strings.HasPrefix(strings.TrimSpace(device), "Wicked ")
	if !heroic && !legendary {
		return items, augments
	}
	type candidate struct {
		kind  string
		index int
		level int
	}
	candidates := make([]candidate, 0, len(items)+len(augments))
	for _, index := range items {
		level, err := strconv.Atoi(strings.TrimSpace(result.Items[index].MinimumLevel))
		if err == nil {
			candidates = append(candidates, candidate{kind: "item", index: index, level: level})
		}
	}
	for _, index := range augments {
		if result.Augments[index].MinimumLevel != nil {
			candidates = append(candidates, candidate{kind: "augment", index: index, level: *result.Augments[index].MinimumLevel})
		}
	}
	if len(candidates) == 0 {
		return items, augments
	}
	target := candidates[0].level
	for _, candidate := range candidates[1:] {
		if heroic && candidate.level < target || legendary && candidate.level > target {
			target = candidate.level
		}
	}
	var filteredItems, filteredAugments []int
	for _, candidate := range candidates {
		if candidate.level != target {
			continue
		}
		if candidate.kind == "item" {
			filteredItems = append(filteredItems, candidate.index)
		} else {
			filteredAugments = append(filteredAugments, candidate.index)
		}
	}
	return filteredItems, filteredAugments
}

func eligibleItem(item dataset.ItemData) bool {
	for _, slot := range item.Augments {
		if strings.HasPrefix(strings.TrimSpace(slot.AugmentType), "Lamordia") {
			return true
		}
	}
	return false
}

func transformItem(record dataset.ItemRecord) (contracts.ViktraniumItem, error) {
	item := record.Item
	if strings.TrimSpace(item.PageTitle) == "" || strings.TrimSpace(item.Name) == "" {
		return contracts.ViktraniumItem{}, fmt.Errorf("pageTitle and name are required")
	}
	out := contracts.ViktraniumItem{
		ID: stableID("item", strings.TrimSpace(item.PageTitle)), Name: item.Name, PageTitle: item.PageTitle,
		Type: item.Type, Description: item.Description, MinimumLevel: item.MinLevel, Material: item.Material,
		Restriction: item.Restriction, Notes: item.Details, Icon: item.Icon, Image: item.Image,
	}
	if item.Binding != nil {
		out.Binding = &contracts.ViktraniumBinding{Type: item.Binding.Type, To: item.Binding.To, From: item.Binding.From}
	}
	for _, effect := range item.Enchantments {
		out.Enchantments = append(out.Enchantments, contracts.ViktraniumEffect{Name: effect.Name, Modifier: effect.Amount, Bonus: effect.BonusType, Notes: stringValue(effect.Notes)})
	}
	seen := make(map[string]struct{})
	for order, slot := range item.Augments {
		typeName := strings.TrimSpace(slot.AugmentType)
		if typeName == "" {
			return contracts.ViktraniumItem{}, fmt.Errorf("slot[%d] has empty canonical augment type", order)
		}
		id := stableID("slot", strings.TrimSpace(item.PageTitle)+"\x00"+strconv.Itoa(order)+"\x00"+typeName)
		if _, exists := seen[id]; exists {
			return contracts.ViktraniumItem{}, fmt.Errorf("slot identity collision %q", id)
		}
		seen[id] = struct{}{}
		out.Slots = append(out.Slots, contracts.ViktraniumSlot{ID: id, AugmentType: typeName, Label: typeName, Order: order})
		if existingName := first(slot.Title, slot.Name); strings.TrimSpace(existingName) != "" {
			out.Slots[len(out.Slots)-1].ExistingAugmentID = "name:" + strings.TrimSpace(existingName)
		}
	}
	for _, drop := range item.DropLocations {
		out.DropLocations = append(out.DropLocations, contracts.ViktraniumDropLocation{
			SourceType: drop.SourceType, Source: first(drop.QuestWildernessChain, drop.AdventurePack, drop.StoreName),
			Location: first(drop.WhichChestPerson, drop.CraftingLocation, drop.VendorLocation),
			Title:    drop.TitleForLink, Difficulty: drop.Difficulty,
		})
	}
	return out, nil
}

func transformAugment(record dataset.AugmentRecord) (contracts.ViktraniumAugment, error) {
	augment := record.Augment
	if strings.TrimSpace(augment.Name) == "" || strings.TrimSpace(augment.AugmentType) == "" {
		return contracts.ViktraniumAugment{}, fmt.Errorf("name and augmentType are required")
	}
	out := contracts.ViktraniumAugment{
		ID: stableID("augment", augmentIdentity(augment)), Name: augment.Name, PageTitle: augment.Title,
		AugmentType: strings.TrimSpace(augment.AugmentType), MinimumLevel: augment.MinLevel,
		Description: augment.Description, FoundIn: sortedCopy(augment.FoundIn), Notes: augment.Notes, Image: augment.Image,
	}
	for _, effect := range append(append([]dataset.PartialEnhancementOut(nil), augment.Enhancements...), augment.EffectsAdded...) {
		modifier := ""
		if effect.Modifier != nil {
			modifier = fmt.Sprint(effect.Modifier)
		}
		out.Effects = append(out.Effects, contracts.ViktraniumEffect{
			Name: effect.Name, Modifier: modifier, Bonus: effect.Bonus,
			Description: effect.Description, Notes: effect.Notes, Damage: sortedCopy(effect.Damage),
		})
	}
	return out, nil
}

func augmentIdentity(augment dataset.AugmentItem) string {
	level := ""
	if augment.MinLevel != nil {
		level = strconv.Itoa(*augment.MinLevel)
	}
	return strings.Join([]string{strings.TrimSpace(augment.Title), strings.TrimSpace(augment.Name), strings.TrimSpace(augment.AugmentType), level}, "\x00")
}

func compatible(slotType, augmentType string) bool {
	return domain.AugmentTypesCompatible(slotType, augmentType)
}

func validateAcyclic(graph map[int64][]int64) error {
	state := make(map[int64]uint8)
	var visit func(int64) error
	visit = func(id int64) error {
		if state[id] == 1 {
			return fmt.Errorf("cyclic recipe relationship at recipe %d", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, child := range graph[id] {
			if err := visit(child); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range graph {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func sortOutput(result *contracts.ViktraniumDataset) {
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].ID < result.Items[j].ID })
	sort.Slice(result.Augments, func(i, j int) bool { return result.Augments[i].ID < result.Augments[j].ID })
	sort.Slice(result.Ingredients, func(i, j int) bool { return result.Ingredients[i].ID < result.Ingredients[j].ID })
	for index := range result.Items {
		sort.Slice(result.Items[index].Recipes, func(i, j int) bool { return result.Items[index].Recipes[i].ID < result.Items[index].Recipes[j].ID })
	}
	for index := range result.Augments {
		sort.Slice(result.Augments[index].Recipes, func(i, j int) bool {
			return result.Augments[index].Recipes[i].ID < result.Augments[index].Recipes[j].ID
		})
	}
}

func stableID(kind, identity string) string {
	digest := sha256.Sum256([]byte(identity))
	return kind + "-" + hex.EncodeToString(digest[:12])
}

func recipeID(id int64) string     { return "recipe-" + strconv.FormatInt(id, 10) }
func ingredientID(id int64) string { return "ingredient-" + strconv.FormatInt(id, 10) }
func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
