package contracts

// EssenceCraftingSchemaVersion is the version of the generated Essence
// Crafting read model.
const EssenceCraftingSchemaVersion = 1

type EssenceCraftingDomain struct {
	SchemaVersion      int                           `json:"schemaVersion"`
	Rules              EssenceCraftingRules          `json:"rules"`
	ItemCategories     []EssenceCraftingNamedRecord  `json:"itemCategories"`
	BonusTypes         []EssenceCraftingNamedRecord  `json:"bonusTypes"`
	AugmentTypes       []EssenceCraftingNamedRecord  `json:"augmentTypes"`
	Enhancements       []EssenceCraftingEnhancement  `json:"enhancements"`
	Recipes            []EssenceCraftingRecipe       `json:"recipes"`
	MinimumLevelShards []EssenceCraftingMinimumShard `json:"minimumLevelShards"`
	Augments           []EssenceCraftingAugment      `json:"augments"`
	Ingredients        []EssenceCraftingIngredient   `json:"ingredients"`
}

type EssenceCraftingNamedRecord struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type EssenceCraftingRules struct {
	SupportedItemLevels   EssenceCraftingLevelRange         `json:"supportedItemLevels"`
	MaximumCraftingLevel  int                               `json:"maximumCraftingLevel"`
	ExtraAffix            EssenceCraftingExtraAffix         `json:"extraAffix"`
	AugmentSlotTypes      []EssenceCraftingAugmentSlotType  `json:"augmentSlotTypes"`
	AugmentSlotPlacements []EssenceCraftingAugmentPlacement `json:"augmentSlotPlacements"`
}

type EssenceCraftingLevelRange struct {
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}

type EssenceCraftingExtraAffix struct {
	Position        string                     `json:"position"`
	MarkRequirement EssenceCraftingRequirement `json:"markRequirement"`
	ConsumedWhen    string                     `json:"consumedWhen"`
}

type EssenceCraftingAugmentSlotType struct {
	ID                    string   `json:"id"`
	DisplayName           string   `json:"displayName"`
	AcceptsAugmentTypeIDs []string `json:"acceptsAugmentTypeIds"`
}

type EssenceCraftingAugmentPlacement struct {
	ItemCategoryID string   `json:"itemCategoryId"`
	SlotTypeIDs    []string `json:"augmentSlotTypeIds"`
}

type EssenceCraftingEnhancement struct {
	ID               string                     `json:"id"`
	DisplayName      string                     `json:"displayName"`
	MinimumItemLevel int                        `json:"minimumItemLevel"`
	Placements       []EssenceCraftingPlacement `json:"placements"`
	Effects          []EssenceCraftingEffect    `json:"effects"`
	Recipes          EssenceCraftingRecipePair  `json:"recipes"`
}

type EssenceCraftingPlacement struct {
	Position        string   `json:"position"`
	ItemCategoryIDs []string `json:"itemCategoryIds"`
}

type EssenceCraftingEffect struct {
	ID          string                   `json:"id"`
	DisplayName string                   `json:"displayName"`
	BonusTypeID string                   `json:"bonusTypeId,omitempty"`
	Modifier    *EssenceCraftingModifier `json:"modifier,omitempty"`
}

type EssenceCraftingModifier struct {
	Kind  string                        `json:"kind"`
	Unit  string                        `json:"unit"`
	Value any                           `json:"value,omitempty"`
	Die   string                        `json:"die,omitempty"`
	Bands []EssenceCraftingModifierBand `json:"bands,omitempty"`
}

type EssenceCraftingModifierBand struct {
	MinimumItemLevel int     `json:"minimumItemLevel"`
	MaximumItemLevel int     `json:"maximumItemLevel"`
	Value            float64 `json:"value"`
}

type EssenceCraftingRecipePair struct {
	BoundRecipeID   string `json:"boundRecipeId"`
	UnboundRecipeID string `json:"unboundRecipeId"`
}

type EssenceCraftingRecipe struct {
	ID             string                       `json:"id"`
	Kind           string                       `json:"kind"`
	SourceRecipeID string                       `json:"sourceRecipeId,omitempty"`
	ItemLevel      int                          `json:"itemLevel,omitempty"`
	Binding        string                       `json:"binding"`
	CraftingLevel  int                          `json:"craftingLevel"`
	Requirements   []EssenceCraftingRequirement `json:"requirements"`
}

type EssenceCraftingRequirement struct {
	Kind         string `json:"kind"`
	IngredientID string `json:"ingredientId,omitempty"`
	RecipeID     string `json:"recipeId,omitempty"`
	Quantity     int    `json:"quantity"`
}

type EssenceCraftingMinimumShard struct {
	ItemLevel int                       `json:"itemLevel"`
	Recipes   EssenceCraftingRecipePair `json:"recipes"`
}

type EssenceCraftingAugment struct {
	ID               string                  `json:"id"`
	DisplayName      string                  `json:"displayName"`
	AugmentTypeID    string                  `json:"augmentTypeId"`
	MinimumItemLevel int                     `json:"minimumItemLevel"`
	Effects          []EssenceCraftingEffect `json:"effects"`
}

type EssenceCraftingIngredient struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}
