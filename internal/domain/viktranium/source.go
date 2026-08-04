package viktranium

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	heroicRecipesFile    = "viktranium.heroic.recipes.json"
	legendaryRecipesFile = "viktranium.legendary.recipes.json"
	wickedRecipesFile    = "viktranium.wicked.recipes.json"
	ingredientsFile      = "ingredients.json"
)

var recipeFiles = [...]string{heroicRecipesFile, legendaryRecipesFile, wickedRecipesFile}

type sourceRecipe struct {
	RecipeID             int64               `json:"recipeId"`
	Name                 string              `json:"name"`
	DeviceID             int64               `json:"deviceId"`
	Device               string              `json:"device"`
	Removed              *string             `json:"removed"`
	Added                *string             `json:"added"`
	Produces             string              `json:"produces"`
	ProductEffect        string              `json:"productEffect"`
	EssenceCraftingLevel *float64            `json:"essenceCraftingLevel"`
	CraftingSchool       *string             `json:"craftingSchool"`
	MinItemLevel         *float64            `json:"minItemLevel"`
	Grants               []string            `json:"grants"`
	Ingredients          []sourceRequirement `json:"ingredients"`
}

type sourceRequirement struct {
	IngredientID int64    `json:"ingredientId"`
	Name         string   `json:"name"`
	Quantity     *float64 `json:"quantity"`
}

type sourceIngredient struct {
	Name           string   `json:"name"`
	IngredientType string   `json:"ingredientType,omitempty"`
	Type           string   `json:"type,omitempty"`
	Description    string   `json:"description,omitempty"`
	FoundIn        []string `json:"foundIn,omitempty"`
	Icon           string   `json:"icon,omitempty"`
	Image          string   `json:"image,omitempty"`
}

type sources struct {
	Recipes     []sourceRecipe
	Ingredients []sourceIngredient
}

// Recipe ingredient IDs are authoritative for generated references. The
// canonical ingredient payload has no ID field, so its established name field
// is the only supported metadata join; zero or multiple name matches fail.
func loadSources(root string) (sources, error) {
	if strings.TrimSpace(root) == "" {
		return sources{}, fmt.Errorf("manual source directory is required")
	}
	var result sources
	seenRecipeIDs := make(map[int64]string)
	for _, name := range recipeFiles {
		var recipes []sourceRecipe
		if err := decodeStrictFile(filepath.Join(root, name), &recipes); err != nil {
			return sources{}, fmt.Errorf("load manual Viktranium file %s: %w", name, err)
		}
		if len(recipes) == 0 {
			return sources{}, fmt.Errorf("manual Viktranium file %s contains no recipes", name)
		}
		for index, recipe := range recipes {
			context := fmt.Sprintf("%s recipe[%d]", name, index)
			if recipe.RecipeID == 0 || strings.TrimSpace(recipe.Name) == "" || recipe.DeviceID == 0 ||
				strings.TrimSpace(recipe.Device) == "" || strings.TrimSpace(recipe.Produces) == "" {
				return sources{}, fmt.Errorf("%s: recipeId, name, deviceId, device, and produces are required", context)
			}
			if previous, exists := seenRecipeIDs[recipe.RecipeID]; exists {
				return sources{}, fmt.Errorf("%s: recipeId %d duplicates %s", context, recipe.RecipeID, previous)
			}
			seenRecipeIDs[recipe.RecipeID] = context
			if len(recipe.Ingredients) == 0 {
				return sources{}, fmt.Errorf("%s: ingredients are required", context)
			}
			for requirementIndex, requirement := range recipe.Ingredients {
				if strings.TrimSpace(requirement.Name) == "" || requirement.IngredientID == 0 {
					return sources{}, fmt.Errorf("%s ingredient[%d]: ingredientId and name are required", context, requirementIndex)
				}
				if err := validateRequirement(requirement); err != nil {
					return sources{}, fmt.Errorf("%s ingredient %q: %w", context, requirement.Name, err)
				}
			}
		}
		result.Recipes = append(result.Recipes, recipes...)
	}
	if err := decodeFile(filepath.Join(root, ingredientsFile), &result.Ingredients); err != nil {
		return sources{}, fmt.Errorf("load canonical ingredients %s: %w", ingredientsFile, err)
	}
	return result, nil
}

func validateRequirement(requirement sourceRequirement) error {
	if requirement.Quantity == nil {
		return fmt.Errorf("quantity is required")
	}
	quantity := *requirement.Quantity
	if math.IsNaN(quantity) || math.IsInf(quantity, 0) || quantity <= 0 || math.Trunc(quantity) != quantity {
		return fmt.Errorf("quantity must be a positive finite integer")
	}
	return nil
}

func decodeStrictFile(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if err := requireEOF(decoder); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func decodeFile(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return requireEOF(decoder)
}

func requireEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}
