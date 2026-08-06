package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yourddo-data-tools/internal/contracts"
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
	first, err := BuildCandidate("81.3.0", "source", "master", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCandidate("81.3.0", "source", "master", root, nil)
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

func TestArtifactFingerprintExcludesGameVersion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeArtifact(t, root, "master/items.json", "[]\n")
	writeArtifact(t, root, "gear-planner/items.json", "[]\n")
	first, err := BuildCandidate("81.3.0", "source", "master", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCandidate("81.3.1", "source", "master", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.ReleaseFingerprint != second.ReleaseFingerprint {
		t.Fatalf("game version changed artifact fingerprint: %s != %s", first.ReleaseFingerprint, second.ReleaseFingerprint)
	}
}

func TestArtifactFingerprintIsDeterministic(t *testing.T) {
	t.Parallel()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	creationOrders := [][]string{
		{"manual/settings.json", "gear-planner/items.json", "master/items.json"},
		{"master/items.json", "gear-planner/items.json", "manual/settings.json"},
	}
	contents := map[string]string{
		"master/items.json":       "[]\n",
		"gear-planner/items.json": "[{\"id\":\"item\"}]\n",
		"manual/settings.json":    "{\"enabled\":true}\n",
	}
	for index, root := range []string{firstRoot, secondRoot} {
		for _, relative := range creationOrders[index] {
			writeArtifact(t, root, relative, contents[relative])
		}
	}
	files := []contracts.GeneratedFileMetadata{{Path: "master/items.json"}, {Path: "gear-planner/items.json"}}
	payloads := []contracts.ManualPayloadMetadata{{Path: "manual/settings.json"}}
	first := artifactFingerprint(t, firstRoot, files, payloads)
	second := artifactFingerprint(t, secondRoot, []contracts.GeneratedFileMetadata{files[1], files[0]}, payloads)
	if first != second {
		t.Fatalf("fingerprints differ by creation order or staging root: %s != %s", first, second)
	}

	modTime := time.Unix(2_000_000_000, 0)
	if err := os.Chtimes(filepath.Join(secondRoot, "gear-planner", "items.json"), modTime, modTime); err != nil {
		t.Fatal(err)
	}
	writeArtifact(t, secondRoot, "unpublished/debug.json", "{\"changed\":true}\n")
	if got := artifactFingerprint(t, secondRoot, files, payloads); got != first {
		t.Fatalf("mtime or unpublished file changed fingerprint: %s != %s", got, first)
	}
}

func TestArtifactFingerprintChangesWithContentAndPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeArtifact(t, root, "domain/items.json", "[]\n")
	writeArtifact(t, root, "renamed/items.json", "[]\n")
	files := []contracts.GeneratedFileMetadata{{Path: "domain/items.json"}}
	original := artifactFingerprint(t, root, files, nil)
	writeArtifact(t, root, "domain/items.json", "[1]\n")
	if changed := artifactFingerprint(t, root, files, nil); changed == original {
		t.Fatal("changing artifact bytes did not change fingerprint")
	}
	if renamed := artifactFingerprint(t, root, []contracts.GeneratedFileMetadata{{Path: "renamed/items.json"}}, nil); renamed == original {
		t.Fatal("changing the publish-relative path did not change fingerprint")
	}
}

func TestArtifactFingerprintDetectsGeneratedDomainChangeWithIdenticalCanonicalInput(t *testing.T) {
	t.Parallel()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	for _, root := range []string{firstRoot, secondRoot} {
		writeArtifact(t, root, "master/items.json", "[{\"id\":\"canonical\"}]\n")
	}
	writeArtifact(t, firstRoot, "gear-planner/items.json", "[{\"id\":\"old-output\"}]\n")
	writeArtifact(t, secondRoot, "gear-planner/items.json", "[{\"id\":\"new-output\"}]\n")
	files := []contracts.GeneratedFileMetadata{{Path: "master/items.json"}, {Path: "gear-planner/items.json"}}
	first := artifactFingerprint(t, firstRoot, files, nil)
	second := artifactFingerprint(t, secondRoot, files, nil)
	if first == second {
		t.Fatal("generated domain output change with identical canonical input did not change fingerprint")
	}
	writeArtifact(t, secondRoot, "gear-planner/items.json", "[{\"id\":\"old-output\"}]\n")
	if rebuilt := artifactFingerprint(t, secondRoot, files, nil); rebuilt != first {
		t.Fatalf("byte-identical generated output changed fingerprint: %s != %s", rebuilt, first)
	}
}

func TestArtifactFingerprintManifestFormat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeArtifact(t, root, "domain/items.json", "[]\n")
	fileDigest := sha256.Sum256([]byte("[]\n"))
	manifestJSON := `{"schemaVersion":7,"files":[{"path":"domain/items.json","sha256":"` + hex.EncodeToString(fileDigest[:]) + `"}]}`
	wantDigest := sha256.Sum256([]byte(manifestJSON))
	want := hex.EncodeToString(wantDigest[:])
	if got := artifactFingerprint(t, root, []contracts.GeneratedFileMetadata{{Path: "domain/items.json"}}, nil); got != want {
		t.Fatalf("artifact fingerprint = %s, want hash of %s = %s", got, manifestJSON, want)
	}
}

func artifactFingerprint(t *testing.T, root string, files []contracts.GeneratedFileMetadata, payloads []contracts.ManualPayloadMetadata) string {
	t.Helper()
	value, err := ArtifactFingerprint(root, files, payloads)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func writeArtifact(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseRequiresPositiveDataVersion(t *testing.T) {
	t.Parallel()
	_, err := Release(Candidate{SchemaVersion: 2, GameVersion: "81.3.0", MasterDatasetSHA256: "master", ReleaseFingerprint: "fingerprint"}, 0)
	if err == nil {
		t.Fatal("Release succeeded with zero data version")
	}
}
