package compendium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/hashing"
)

type fixtureSource struct {
	categories map[string]map[string]string
	pages      map[string]string
}

func (f fixtureSource) FetchCategoryContent(_ context.Context, category string) (map[string]string, error) {
	pages, ok := f.categories[category]
	if !ok {
		return nil, errors.New("unknown fixture category")
	}
	result := make(map[string]string, len(pages))
	maps.Copy(result, pages)
	return result, nil
}

func (f fixtureSource) FetchPageContent(_ context.Context, title string) (string, error) {
	content, ok := f.pages[title]
	if !ok {
		return "", errors.New("unknown fixture page")
	}
	return content, nil
}

func loadFixture(t *testing.T, name string) fixtureSource {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Categories map[string]map[string]string `json:"categories"`
		Pages      map[string]string            `json:"pages"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return fixtureSource{categories: raw.Categories, pages: raw.Pages}
}

func TestGeneratorParsesKnownRecordsAndSortsCanonically(t *testing.T) {
	t.Parallel()
	generator, err := NewGenerator(loadFixture(t, "source-records.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "master")
	result, err := generator.Generate(context.Background(), []string{"Test Items", "Filigree Sets", "Augment"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SHA256) != 64 || result.OutputRoot != root || len(result.Master.Files) == 0 ||
		len(result.Master.Items) != 2 || len(result.Master.Augments) != 2 {
		t.Fatalf("result = %#v", result)
	}
	actualHash, err := hashing.Directory(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != actualHash {
		t.Fatalf("returned hash %q, directory hash %q", result.SHA256, actualHash)
	}
	loadedMaster, err := dataset.LoadMaster(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Master, loadedMaster) {
		t.Fatal("master generator result differs from the standalone domain input contract")
	}

	var items []dataset.ItemData
	if err := dataset.ReadJSON(filepath.Join(root, "testItems.json"), &items); err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(items))
	for index := range items {
		names[index] = items[index].Name
	}
	if want := []string{"Item 2", "Item 10"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("item order = %v, want %v", names, want)
	}
	if len(items[0].DropLocations) != 2 || items[0].DropLocations[0].QuestWildernessChain != "Quest A" {
		t.Fatalf("drop locations were not canonically sorted: %#v", items[0].DropLocations)
	}
	if len(items[1].Enchantments) != 2 || items[1].Enchantments[0].Name != "Strength" {
		t.Fatalf("meaningful enchantment order was not preserved: %#v", items[1].Enchantments)
	}
	// Pre-update, discontinued, and starter records are explicit master-source
	// exclusions inherited from dataSpider.
	var augments []dataset.AugmentItem
	if err := dataset.ReadJSON(filepath.Join(root, "augment.json"), &augments); err != nil {
		t.Fatal(err)
	}
	if len(augments) != 2 || augments[0].Name != "Ruby of Testing 2" || augments[1].Name != "Ruby of Testing 10" {
		t.Fatalf("augment master exclusions or ordering are wrong: %#v", augments)
	}

	var sets []dataset.FiligreeSet
	if err := dataset.ReadJSON(filepath.Join(root, "filigreeSets.json"), &sets); err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].Name != "Fixture Set 2" || sets[1].Bonuses[0].Threshold != 2 {
		t.Fatalf("filigree sets were not canonically sorted: %#v", sets)
	}
}

func TestGeneratorRejectsDuplicateCanonicalIdentifiers(t *testing.T) {
	t.Parallel()
	source := fixtureSource{categories: map[string]map[string]string{
		"Rings": {"Shared Page": "{{Item|name=First|type=Ring}}"},
		"Belts": {"Shared Page": "{{Item|name=Second|type=Belt}}"},
	}}
	generator, err := NewGenerator(source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = generator.Generate(context.Background(), []string{"Rings", "Belts"}, filepath.Join(t.TempDir(), "master"))
	if err == nil || !strings.Contains(err.Error(), `duplicate canonical identifier "items:shared page"`) ||
		!strings.Contains(err.Error(), "Rings/Shared Page") || !strings.Contains(err.Error(), "Belts/Shared Page") {
		t.Fatalf("error = %v", err)
	}
}

func TestGeneratorReportsMalformedSourceRecord(t *testing.T) {
	t.Parallel()
	generator, err := NewGenerator(loadFixture(t, "malformed-records.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = generator.Generate(context.Background(), []string{"Broken Items"}, filepath.Join(t.TempDir(), "master"))
	if err == nil || !strings.Contains(err.Error(), `source record "Broken Source Record"`) ||
		!strings.Contains(err.Error(), "no matching closing") {
		t.Fatalf("error = %v", err)
	}
}

func TestGeneratorReportsMalformedTypedField(t *testing.T) {
	t.Parallel()
	generator, err := NewGenerator(loadFixture(t, "malformed-records.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = generator.Generate(context.Background(), []string{"Augment"}, filepath.Join(t.TempDir(), "master"))
	if err == nil || !strings.Contains(err.Error(), `source record "Broken Augment Record"`) ||
		!strings.Contains(err.Error(), `field "minlevel"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestGeneratorSerializationAndHashAreDeterministic(t *testing.T) {
	t.Parallel()
	source := loadFixture(t, "source-records.json")
	generator, err := NewGenerator(source)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	first, err := generator.Generate(context.Background(), []string{"Test Items", "Augment"}, filepath.Join(parent, "first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := generator.Generate(context.Background(), []string{"Augment", "Test Items"}, filepath.Join(parent, "second"))
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != second.SHA256 {
		t.Fatalf("identical canonical input hashes differ: %s != %s", first.SHA256, second.SHA256)
	}
	assertDirectoriesEqual(t, first.OutputRoot, second.OutputRoot)

	changedSource := loadFixture(t, "source-records.json")
	changedSource.categories["Test Items"]["Item 2"] = strings.Replace(
		changedSource.categories["Test Items"]["Item 2"], "minlevel=2", "minlevel=3", 1,
	)
	changedGenerator, err := NewGenerator(changedSource)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := changedGenerator.Generate(context.Background(), []string{"Test Items", "Augment"}, filepath.Join(parent, "changed"))
	if err != nil {
		t.Fatal(err)
	}
	if changed.SHA256 == first.SHA256 {
		t.Fatal("different canonical data produced the same hash")
	}
}

func TestGeneratorNeverOverwritesExistingOutput(t *testing.T) {
	t.Parallel()
	generator, err := NewGenerator(loadFixture(t, "source-records.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "master")
	if _, err := generator.Generate(context.Background(), []string{"Test Items"}, root); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "master-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.Generate(context.Background(), []string{"Test Items"}, root); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("second generation error = %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "master-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing canonical output changed")
	}
}

func TestGeneratorReplacingSwapsLocalOutput(t *testing.T) {
	t.Parallel()
	source := loadFixture(t, "source-records.json")
	generator, err := NewGenerator(source)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "master")
	first, err := generator.GenerateReplacing(context.Background(), []string{"Test Items"}, root)
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, "stale-file")
	if err := os.WriteFile(stale, []byte("old dataset"), 0o644); err != nil {
		t.Fatal(err)
	}

	changedSource := loadFixture(t, "source-records.json")
	changedSource.categories["Test Items"]["Item 2"] = strings.Replace(
		changedSource.categories["Test Items"]["Item 2"], "minlevel=2", "minlevel=3", 1,
	)
	changedGenerator, err := NewGenerator(changedSource)
	if err != nil {
		t.Fatal(err)
	}
	second, err := changedGenerator.GenerateReplacing(context.Background(), []string{"Test Items"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 == second.SHA256 {
		t.Fatal("replacement did not change the canonical hash")
	}
	if contents, err := os.ReadFile(stale); err != nil || string(contents) != "old dataset" {
		t.Fatalf("unrelated local output was not preserved: contents=%q error=%v", contents, err)
	}
	var items []dataset.ItemData
	if err := dataset.ReadJSON(filepath.Join(root, "testItems.json"), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || items[0].MinLevel != "3" {
		t.Fatalf("replacement data was not promoted: %#v", items)
	}
	actualHash, err := hashing.Directory(root)
	if err != nil {
		t.Fatal(err)
	}
	if second.SHA256 != actualHash {
		t.Fatalf("replacement hash %q, directory hash %q", second.SHA256, actualHash)
	}
}

func TestGeneratorReplacingMergesMasterIndexAcrossCategoryRuns(t *testing.T) {
	t.Parallel()
	source := fixtureSource{categories: map[string]map[string]string{
		"Ring": {"Test Ring": "{{Item|name=Test Ring|type=Ring|minlevel=1}}"},
		"Dart": {"Test Dart": "{{Item|name=Test Dart|type=Dart|minlevel=1}}"},
	}}
	generator, err := NewGenerator(source)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "master")
	if _, err := generator.GenerateReplacing(context.Background(), []string{"Ring"}, root); err != nil {
		t.Fatal(err)
	}
	if _, err := generator.GenerateReplacing(context.Background(), []string{"Dart"}, root); err != nil {
		t.Fatal(err)
	}

	index, err := dataset.LoadMasterIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []dataset.MasterFile{
		{Category: "Dart", Kind: "items", Path: "dart.json"},
		{Category: "Ring", Kind: "items", Path: "ring.json"},
	}; !reflect.DeepEqual(index.Files, want) {
		t.Fatalf("merged master index = %#v, want %#v", index.Files, want)
	}
	for _, relative := range []string{"dart.json", "ring.json"} {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			t.Fatalf("indexed item data file %s: %v", relative, err)
		}
	}
}

func TestGeneratorEmitsCollarAndRuneArmCategories(t *testing.T) {
	t.Parallel()
	source := fixtureSource{categories: map[string]map[string]string{
		"Collar": {
			"Test Collar": "{{Template:Armor\n|name=Test Collar\n|type=Collar\n|minlevel=1\n}}",
		},
		"Rune Arm": {
			"Test Rune Arm": "{{Template:RuneArm\n|name=Test Rune Arm\n|type=Rune Arm\n|minlevel=5\n}}",
		},
	}}
	generator, err := NewGenerator(source)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "master")
	result, err := generator.Generate(context.Background(), []string{"Collar", "Rune Arm"}, root)
	if err != nil {
		t.Fatal(err)
	}

	wantFiles := []dataset.MasterFile{
		{Category: "Collar", Kind: "items", Path: "collar.json"},
		{Category: "Rune Arm", Kind: "items", Path: "runeArm.json"},
	}
	if !reflect.DeepEqual(result.Master.Index.Files, wantFiles) {
		t.Fatalf("master files = %#v, want %#v", result.Master.Index.Files, wantFiles)
	}
	wantTypes := map[string]string{"collar.json": "Collar", "runeArm.json": "Rune Arm"}
	for relative, wantType := range wantTypes {
		var items []dataset.ItemData
		if err := dataset.ReadJSON(filepath.Join(root, relative), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Type != wantType {
			t.Fatalf("%s items = %#v, want one item with type %q", relative, items, wantType)
		}
	}
}

func TestExpandCategoriesAllUsesEveryConcreteSourceCategory(t *testing.T) {
	t.Parallel()
	got := expandCategories([]string{"all"})
	if !sort.SliceIsSorted(got, func(i, j int) bool {
		return dataset.NaturalLess(got[i], got[j])
	}) {
		t.Fatalf("All categories are not alphabetically sorted: %v", got)
	}
	seen := make(map[string]bool, len(got))
	for _, category := range got {
		if seen[category] {
			t.Fatalf("All contains duplicate category %q", category)
		}
		seen[category] = true
	}
	for _, required := range []string{
		"Augment", "Collar", "Rune Arm", "Docent", "Ring", "Falchion", "Dart", "Quiver", "Filigrees", "Filigree Sets",
	} {
		if !seen[required] {
			t.Errorf("All does not include %q", required)
		}
	}
	for aggregate, concrete := range aggregateCategories {
		isConcrete := false
		for _, category := range concrete {
			isConcrete = isConcrete || category == aggregate
		}
		if !isConcrete && seen[aggregate] {
			t.Errorf("All contains aggregate name %q instead of only its concrete categories", aggregate)
		}
	}
}

func assertDirectoriesEqual(t *testing.T, first, second string) {
	t.Helper()
	firstPaths, err := hashing.Files(first)
	if err != nil {
		t.Fatal(err)
	}
	secondPaths, err := hashing.Files(second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstPaths, secondPaths) {
		t.Fatalf("paths differ: %v != %v", firstPaths, secondPaths)
	}
	for _, relative := range firstPaths {
		firstData, err := os.ReadFile(filepath.Join(first, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		secondData, err := os.ReadFile(filepath.Join(second, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(firstData, secondData) {
			t.Errorf("%s differs", relative)
		}
	}
}
