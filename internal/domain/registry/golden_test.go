package registry_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/domain/registry"
	"yourddo-data-tools/internal/hashing"
	"yourddo-data-tools/internal/validation"
)

func TestGeneratorsMatchGoldenFilesAndAreByteDeterministic(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "..", "testdata")
	master, err := dataset.LoadMaster(filepath.Join(fixtureRoot, "master"))
	if err != nil {
		t.Fatal(err)
	}
	firstRoot := filepath.Join(t.TempDir(), "first")
	secondRoot := filepath.Join(t.TempDir(), "second")
	var firstFiles, secondFiles int
	for _, registration := range registry.All() {
		first, err := registration.Generator.Generate(context.Background(), master, firstRoot)
		if err != nil {
			t.Fatalf("generate %s first run: %v", registration.Generator.Name(), err)
		}
		second, err := registration.Generator.Generate(context.Background(), master, secondRoot)
		if err != nil {
			t.Fatalf("generate %s second run: %v", registration.Generator.Name(), err)
		}
		if !reflect.DeepEqual(first.Files, second.Files) || !reflect.DeepEqual(first.Warnings, second.Warnings) {
			t.Fatalf("domain %s returned nondeterministic results", registration.Generator.Name())
		}
		if len(first.Warnings) != 0 {
			t.Fatalf("domain %s warnings = %v", registration.Generator.Name(), first.Warnings)
		}
		if err := validation.GeneratedFiles(firstRoot, first.Files); err != nil {
			t.Fatalf("validate %s metadata: %v", registration.Generator.Name(), err)
		}
		firstFiles += len(first.Files)
		secondFiles += len(second.Files)
	}
	if firstFiles == 0 || secondFiles != firstFiles {
		t.Fatalf("generated file counts = %d and %d", firstFiles, secondFiles)
	}
	assertTreesEqual(t, firstRoot, secondRoot)

	expectedRoot := filepath.Join(fixtureRoot, "expected")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		writeGoldenTree(t, firstRoot, expectedRoot)
	}
	assertTreesEqual(t, expectedRoot, firstRoot)
}

func TestFilteredDomainErrorNamesDomainAndRecord(t *testing.T) {
	t.Parallel()
	generator, err := registry.Resolve("almost-there")
	if err != nil {
		t.Fatal(err)
	}
	master := dataset.Master{Items: []dataset.ItemRecord{{
		File: "rings.json", Item: itemWithEnchantment("Almost There"),
	}}}
	_, err = generator.Generate(context.Background(), master, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "domain almost-there") || !strings.Contains(err.Error(), "rings.json#<unnamed>") {
		t.Fatalf("error = %v", err)
	}
}

func TestRegistryNamesAreUniqueAndResolvable(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{})
	for _, name := range registry.Names() {
		if _, exists := seen[name]; exists {
			t.Fatalf("duplicate registered domain %q", name)
		}
		seen[name] = struct{}{}
		generator, err := registry.Resolve(name)
		if err != nil || generator.Name() != name {
			t.Fatalf("resolve %q = %v, %v", name, generator, err)
		}
	}
}

func TestResolveAllExpandsInRegistrationOrderAndDeduplicates(t *testing.T) {
	t.Parallel()
	generators, err := registry.ResolveAll([]string{"dinosaur-bone", "all", "gearplanner"})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(generators))
	for index, generator := range generators {
		names[index] = generator.Name()
	}
	want := append([]string{"dinosaur-bone"}, registry.Names()...)
	want = removeDuplicate(want)
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("expanded names = %v, want %v", names, want)
	}
}

func removeDuplicate(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func itemWithEnchantment(name string) dataset.ItemData {
	return dataset.ItemData{Enchantments: []dataset.Enchantment{{Name: name}}}
}

func assertTreesEqual(t *testing.T, expectedRoot, actualRoot string) {
	t.Helper()
	expectedPaths, err := hashing.Files(expectedRoot)
	if err != nil {
		t.Fatal(err)
	}
	actualPaths, err := hashing.Files(actualRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expectedPaths, actualPaths) {
		t.Fatalf("file paths differ\nexpected: %v\nactual:   %v", expectedPaths, actualPaths)
	}
	for _, relative := range expectedPaths {
		expected, err := os.ReadFile(filepath.Join(expectedRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(filepath.Join(actualRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(expected, actual) {
			t.Errorf("golden mismatch: %s", relative)
		}
	}
}

func writeGoldenTree(t *testing.T, sourceRoot, expectedRoot string) {
	t.Helper()
	paths, err := hashing.Files(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range paths {
		data, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(expectedRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
