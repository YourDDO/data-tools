package contracts

// ViktraniumSchemaVersion identifies the fully materialized Viktranium payload.
const ViktraniumSchemaVersion = 1

type ViktraniumDataset struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Items         []ViktraniumItem       `json:"items"`
	Augments      []ViktraniumAugment    `json:"augments"`
	Ingredients   []ViktraniumIngredient `json:"ingredients,omitempty"`
}

type ViktraniumItem struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	PageTitle     string                   `json:"pageTitle"`
	Type          string                   `json:"type"`
	Description   string                   `json:"description,omitempty"`
	MinimumLevel  string                   `json:"minimumLevel,omitempty"`
	Binding       *ViktraniumBinding       `json:"binding,omitempty"`
	Material      string                   `json:"material,omitempty"`
	BaseEffects   []ViktraniumEffect       `json:"baseEffects,omitempty"`
	Enchantments  []ViktraniumEffect       `json:"enchantments,omitempty"`
	Slots         []ViktraniumSlot         `json:"slots"`
	Recipes       []ViktraniumRecipe       `json:"recipes,omitempty"`
	Restriction   string                   `json:"restriction,omitempty"`
	Notes         string                   `json:"notes,omitempty"`
	DropLocations []ViktraniumDropLocation `json:"dropLocations,omitempty"`
	Icon          string                   `json:"icon,omitempty"`
	Image         string                   `json:"image,omitempty"`
}

type ViktraniumSlot struct {
	ID                string `json:"id"`
	AugmentType       string `json:"augmentType"`
	Label             string `json:"label"`
	Order             int    `json:"order"`
	ExistingAugmentID string `json:"existingAugmentId,omitempty"`
}

type ViktraniumAugment struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	PageTitle    string             `json:"pageTitle,omitempty"`
	AugmentType  string             `json:"augmentType"`
	MinimumLevel *int               `json:"minimumLevel,omitempty"`
	Description  string             `json:"description,omitempty"`
	Effects      []ViktraniumEffect `json:"effects,omitempty"`
	Recipes      []ViktraniumRecipe `json:"recipes,omitempty"`
	FoundIn      []string           `json:"foundIn,omitempty"`
	Restrictions string             `json:"restrictions,omitempty"`
	Notes        string             `json:"notes,omitempty"`
	Icon         string             `json:"icon,omitempty"`
	Image        string             `json:"image,omitempty"`
}

type ViktraniumIngredient struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	IngredientType string   `json:"ingredientType,omitempty"`
	Description    string   `json:"description,omitempty"`
	FoundIn        []string `json:"foundIn,omitempty"`
	Icon           string   `json:"icon,omitempty"`
	Image          string   `json:"image,omitempty"`
}

type ViktraniumRecipe struct {
	ID            string                  `json:"id"`
	DeviceID      string                  `json:"deviceId"`
	Device        string                  `json:"device"`
	ProductEffect string                  `json:"productEffect,omitempty"`
	Notes         string                  `json:"notes,omitempty"`
	Requirements  []ViktraniumRequirement `json:"requirements"`
}

// ViktraniumRequirement retains the direct recipe tree. Recipe references are
// expanded through RecipeID by clients when calculating cumulative quantities.
type ViktraniumRequirement struct {
	IngredientID string `json:"ingredientId,omitempty"`
	RecipeID     string `json:"recipeId,omitempty"`
	Name         string `json:"name"`
	Quantity     int    `json:"quantity"`
}

type ViktraniumEffect struct {
	Name        string   `json:"name"`
	Modifier    string   `json:"modifier,omitempty"`
	Bonus       string   `json:"bonus,omitempty"`
	Description string   `json:"description,omitempty"`
	Notes       string   `json:"notes,omitempty"`
	Damage      []string `json:"damage,omitempty"`
}

type ViktraniumBinding struct {
	Type string `json:"type"`
	To   string `json:"to,omitempty"`
	From string `json:"from,omitempty"`
}

type ViktraniumDropLocation struct {
	SourceType string `json:"sourceType"`
	Source     string `json:"source,omitempty"`
	Location   string `json:"location,omitempty"`
	Title      string `json:"title,omitempty"`
	Difficulty string `json:"difficulty,omitempty"`
}
