package pipeline

import (
	"context"
	"path/filepath"
	"testing"

	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/manifest"
	"yourddo-data-tools/internal/validation"
)

func TestViktraniumGenerationRegistersExactlyOneManifestFile(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "testdata", "viktranium")
	master, err := dataset.LoadMaster(filepath.Join(fixtureRoot, "master"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	generated, err := GenerateDomains(context.Background(), GenerateOptions{
		Master: master, ManualRoot: filepath.Join(fixtureRoot, "manual"),
		OutputRoot: root, Domains: []string{"viktranium"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validation.GeneratedFiles(root, generated.Files); err != nil {
		t.Fatal(err)
	}
	candidate, err := manifest.BuildCandidate("81.3.0", "source", "master", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, file := range candidate.GeneratedFiles {
		if file.Domain == "viktranium" {
			count++
			if file.Path != "viktranium/viktranium.json" || file.SizeBytes <= 0 || file.SHA256 == "" {
				t.Fatalf("Viktranium metadata = %#v", file)
			}
		}
	}
	if count != 1 || len(candidate.Domains) != 1 || candidate.Domains[0].Domain != "viktranium" || candidate.Domains[0].FileCount != 1 {
		t.Fatalf("candidate = %#v", candidate)
	}
}
