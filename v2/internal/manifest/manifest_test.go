package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateIsDeterministicAndHasNoPublicationTimestamp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, path := range []string{
		filepath.Join(root, "master", "items.json"),
		filepath.Join(root, "gear-planner", "setBonusIndex.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	first, err := BuildCandidate("81.3.0", "source", "master", root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCandidate("81.3.0", "source", "master", root)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("candidate metadata is not deterministic:\n%s\n%s", firstJSON, secondJSON)
	}
	if strings.Contains(string(firstJSON), "dataVersion") || strings.Contains(string(firstJSON), "generatedAt") {
		t.Fatalf("candidate metadata contains a publication or generation timestamp: %s", firstJSON)
	}
	if len(first.Domains) != 2 || first.Domains[0].Domain != "gear-planner" || first.Domains[1].Domain != "master" {
		t.Fatalf("domains = %#v", first.Domains)
	}
}

func TestReleaseRequiresPositiveDataVersion(t *testing.T) {
	t.Parallel()
	_, err := Release(Candidate{SchemaVersion: 1, GameVersion: "81.3.0", MasterDatasetSHA256: "master"}, 0)
	if err == nil {
		t.Fatal("Release succeeded with zero data version")
	}
}
