package viktranium

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
)

func TestLoadSourcesRequiresAllThreeStrictRecipeFiles(t *testing.T) {
	root := writeSourceFixture(t)
	loaded, err := loadSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Recipes) != 3 {
		t.Fatalf("recipe count = %d", len(loaded.Recipes))
	}

	for _, test := range []struct {
		name    string
		file    string
		content string
		want    string
	}{
		{"missing file", wickedRecipesFile, "", "read"},
		{"invalid JSON", heroicRecipesFile, "{", "decode"},
		{"missing required field", legendaryRecipesFile, recipeJSON(2, "", "Lamordia Test"), "required"},
		{"unknown field", heroicRecipesFile, `[{"recipeId":1,"bogus":true}]`, "unknown field"},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyRoot := writeSourceFixture(t)
			path := filepath.Join(copyRoot, test.file)
			if test.content == "" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := loadSources(copyRoot)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestItemDiscoveryPreservesEveryCanonicalSlot(t *testing.T) {
	master, source := validBuildInput()
	master.Items = append(master.Items,
		dataset.ItemRecord{File: "items.json", Item: dataset.ItemData{PageTitle: "Ordinary", Name: "Ordinary", Type: "Ring", Augments: []dataset.AugmentItem{{AugmentType: "Red"}}}},
	)
	got, err := build(context.Background(), master, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || len(got.Items[0].Slots) != 2 {
		t.Fatalf("items/slots = %d/%d", len(got.Items), len(got.Items[0].Slots))
	}
	if got.Items[0].Slots[0].AugmentType != "Red" || got.Items[0].Slots[1].AugmentType != "Lamordia: Melancholic (Weapon)" {
		t.Fatalf("slots = %#v", got.Items[0].Slots)
	}
	if got.Items[0].Slots[0].Order != 0 || got.Items[0].Slots[1].Order != 1 || got.Items[0].Slots[0].ID == got.Items[0].Slots[1].ID {
		t.Fatalf("slot identities/order = %#v", got.Items[0].Slots)
	}
}

func TestDiscoveryAndIdentitiesAreStableAcrossSourceOrdering(t *testing.T) {
	master, source := validBuildInput()
	first, err := build(context.Background(), master, source)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(master.Items)
	slices.Reverse(master.Augments)
	slices.Reverse(source.Recipes)
	second, err := build(context.Background(), master, source)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("source ordering changed generated output")
	}
	if first.Items[0].ID == "" || first.Augments[0].ID == "" {
		t.Fatal("generated identities are empty")
	}
}

func TestDuplicateAugmentNamesRetainDistinctIdentities(t *testing.T) {
	master, source := validBuildInput()
	level := 18
	master.Items[0].Item.Augments = append(master.Items[0].Item.Augments,
		dataset.AugmentItem{AugmentType: "Lamordia: Dolorous (Weapon)"},
		dataset.AugmentItem{AugmentType: "Lamordia: Miserable (Weapon)"},
	)
	master.Augments = append(master.Augments,
		dataset.AugmentRecord{File: "augment.json", Augment: dataset.AugmentItem{Name: "Same Display", Title: "Same Display A", AugmentType: "Lamordia: Dolorous (Weapon)", MinLevel: &level}},
		dataset.AugmentRecord{File: "augment.json", Augment: dataset.AugmentItem{Name: "Same Display", Title: "Same Display B", AugmentType: "Lamordia: Miserable (Weapon)", MinLevel: &level}},
	)
	got, err := build(context.Background(), master, source)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, augment := range got.Augments {
		if augment.Name == "Same Display" {
			ids = append(ids, augment.ID)
		}
	}
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("duplicate-name IDs = %v", ids)
	}
}

func TestRecipeDeviceTierDisambiguatesCanonicalAugmentVariants(t *testing.T) {
	master, source := validBuildInput()
	legendaryLevel := 34
	master.Augments = append(master.Augments, dataset.AugmentRecord{File: "augment.json", Augment: dataset.AugmentItem{
		Name: "Lamordia Test", Title: "Lamordia Test Legendary", AugmentType: "Lamordia: Melancholic (Weapon)", MinLevel: &legendaryLevel,
	}})
	source.Recipes = append(source.Recipes, testRecipe(4, "Lamordia Test", "Heroic Viktranium Experiment Crafting", 5))
	got, err := build(context.Background(), master, source)
	if err != nil {
		t.Fatal(err)
	}
	levels := make(map[int]int)
	for _, augment := range got.Augments {
		if augment.Name == "Lamordia Test" {
			levels[*augment.MinimumLevel] = len(augment.Recipes)
		}
	}
	if levels[18] != 1 || levels[34] != 1 {
		t.Fatalf("tiered recipe counts = %v", levels)
	}
}

func TestIdentityCollisionIsRejected(t *testing.T) {
	master, source := validBuildInput()
	master.Items = append(master.Items, master.Items[0])
	_, err := build(context.Background(), master, source)
	if err == nil || !strings.Contains(err.Error(), "item identity collision") {
		t.Fatalf("error = %v", err)
	}

	master, source = validBuildInput()
	master.Augments = append(master.Augments, master.Augments[0])
	_, err = build(context.Background(), master, source)
	if err == nil || !strings.Contains(err.Error(), "augment identity collision") {
		t.Fatalf("error = %v", err)
	}
}

func TestSlotsRejectEmptyAndZeroCompatibleOptions(t *testing.T) {
	for _, test := range []struct {
		name     string
		typeName string
		want     string
	}{
		{"empty", "", "empty canonical augment type"},
		{"unknown", "Unknown Slot", "no compatible augment"},
	} {
		t.Run(test.name, func(t *testing.T) {
			master, source := validBuildInput()
			master.Items[0].Item.Augments = append(master.Items[0].Item.Augments, dataset.AugmentItem{AugmentType: test.typeName})
			_, err := build(context.Background(), master, source)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestRecipesJoinItemsAugmentsIngredientsAndDevices(t *testing.T) {
	master, source := validBuildInput()
	got, err := build(context.Background(), master, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items[0].Recipes) != 1 || len(got.Augments) != 2 || len(got.Ingredients) != 1 {
		t.Fatalf("output counts = items recipes %d, augments %d, ingredients %d", len(got.Items[0].Recipes), len(got.Augments), len(got.Ingredients))
	}
	if got.Items[0].Recipes[0].Device != "Heroic Device" || got.Items[0].Recipes[0].Requirements[0].Quantity != 5 {
		t.Fatalf("item recipe = %#v", got.Items[0].Recipes[0])
	}
	for _, augment := range got.Augments {
		if len(augment.Recipes) != 1 {
			t.Fatalf("augment recipe missing: %#v", augment)
		}
	}
	if got.Ingredients[0].Description != "Canonical wire" || got.Ingredients[0].Name != "Bleak Wire" {
		t.Fatalf("ingredient = %#v", got.Ingredients[0])
	}
}

func TestRequirementValidationAndAmbiguousIngredientJoin(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*sources)
		want   string
	}{
		{"missing", func(source *sources) { source.Ingredients = nil }, "0 canonical joins"},
		{"ambiguous", func(source *sources) { source.Ingredients = append(source.Ingredients, source.Ingredients[0]) }, "2 canonical joins"},
	} {
		t.Run(test.name, func(t *testing.T) {
			master, source := validBuildInput()
			test.mutate(&source)
			_, err := build(context.Background(), master, source)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	quantityCases := []struct {
		name  string
		value *float64
	}{
		{"missing", nil}, {"zero", floatPointer(0)}, {"negative", floatPointer(-1)},
		{"NaN", floatPointer(math.NaN())}, {"infinity", floatPointer(math.Inf(1))},
	}
	for _, test := range quantityCases {
		t.Run(test.name, func(t *testing.T) {
			requirement := sourceRequirement{IngredientID: 1, Name: "Wire", Quantity: test.value}
			err := validateRequirement(requirement)
			if err == nil {
				t.Fatal("invalid quantity accepted")
			}
		})
	}
}

func TestNestedRequirementsAndCycleDetection(t *testing.T) {
	if err := validateAcyclic(map[int64][]int64{1: {2}, 2: {3}}); err != nil {
		t.Fatal(err)
	}
	if err := validateAcyclic(map[int64][]int64{1: {2}, 2: {1}}); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("cycle error = %v", err)
	}

	master, source := validBuildInput()
	// The item recipe directly requires the augment recipe, preserving quantity.
	source.Recipes[0].Ingredients = []sourceRequirement{{IngredientID: 999, Name: "Lamordia Test", Quantity: floatPointer(2)}}
	got, err := build(context.Background(), master, source)
	if err != nil {
		t.Fatal(err)
	}
	requirement := got.Items[0].Recipes[0].Requirements[0]
	if requirement.RecipeID != "recipe-2" || requirement.Quantity != 2 || requirement.IngredientID != "" {
		t.Fatalf("nested requirement = %#v", requirement)
	}
}

func TestGenerateIsByteDeterministicAndWritesOneFile(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "..", "testdata", "viktranium")
	master, err := dataset.LoadMaster(filepath.Join(fixtureRoot, "master"))
	if err != nil {
		t.Fatal(err)
	}
	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	first, err := New().GenerateWithManual(context.Background(), master, filepath.Join(fixtureRoot, "manual"), firstRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New().GenerateWithManual(context.Background(), master, filepath.Join(fixtureRoot, "manual"), secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != 1 || first.Files[0].Domain != Name || first.Files[0].Path != "viktranium/viktranium.json" {
		t.Fatalf("files = %#v", first.Files)
	}
	firstData, err := os.ReadFile(filepath.Join(firstRoot, filepath.FromSlash(first.Files[0].Path)))
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := os.ReadFile(filepath.Join(secondRoot, filepath.FromSlash(second.Files[0].Path)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstData, secondData) || first.Files[0].SHA256 != second.Files[0].SHA256 {
		t.Fatal("repeat generation was not byte-identical")
	}
	var payload contracts.ViktraniumDataset
	if err := json.Unmarshal(firstData, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != contracts.ViktraniumSchemaVersion {
		t.Fatalf("schema version = %d", payload.SchemaVersion)
	}
}

func validBuildInput() (dataset.Master, sources) {
	level := 18
	master := dataset.Master{
		Items: []dataset.ItemRecord{{File: "weapons.json", Item: dataset.ItemData{
			PageTitle: "Calamitous Test", Name: "Calamitous Test", Type: "Long Sword", MinLevel: "18",
			Augments: []dataset.AugmentItem{{AugmentType: "Red"}, {AugmentType: "Lamordia: Melancholic (Weapon)"}},
		}}},
		Augments: []dataset.AugmentRecord{
			{File: "augment.json", Augment: dataset.AugmentItem{Name: "Lamordia Test", Title: "Lamordia Test", AugmentType: "Lamordia: Melancholic (Weapon)", MinLevel: &level}},
			{File: "augment.json", Augment: dataset.AugmentItem{Name: "Ruby Test", Title: "Ruby Test", AugmentType: "Red", MinLevel: &level}},
		},
	}
	source := sources{
		Recipes: []sourceRecipe{
			testRecipe(1, "Calamitous Test", "Heroic Device", 5),
			testRecipe(2, "Lamordia Test", "Legendary Device", 10),
			testRecipe(3, "Ruby Test", "Wicked Device", 15),
		},
		Ingredients: []sourceIngredient{{Name: "Bleak Wire", IngredientType: "Viktranium", Description: "Canonical wire"}},
	}
	return master, source
}

func testRecipe(id int64, product, device string, quantity float64) sourceRecipe {
	return sourceRecipe{
		RecipeID: id, Name: product, DeviceID: id + 100, Device: device, Produces: product,
		Ingredients: []sourceRequirement{{IngredientID: 500, Name: "Bleak Wire", Quantity: &quantity}},
	}
}

func writeSourceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for index, file := range recipeFiles {
		content := recipeJSON(int64(index+1), "Product "+string(rune('A'+index)), "Product "+string(rune('A'+index)))
		if err := os.WriteFile(filepath.Join(root, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ingredientsFile), []byte(`[{"name":"Bleak Wire"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func recipeJSON(id int64, name, product string) string {
	return `[{"recipeId":` + strconv.FormatInt(id, 10) + `,"name":"` + name + `","deviceId":10,"device":"Device","removed":null,"added":null,"produces":"` + product + `","productEffect":"","essenceCraftingLevel":null,"craftingSchool":null,"minItemLevel":null,"grants":[],"ingredients":[{"ingredientId":100,"name":"Bleak Wire","quantity":1}]}]`
}

func floatPointer(value float64) *float64 { return &value }
