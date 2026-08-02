package validation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/hashing"
	"yourddo-data-tools/internal/manifest"
)

func TestJSONDirectoryReportsInvalidJSONAndMissingNewline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := JSONDirectory(root)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") || !strings.Contains(err.Error(), "newline") {
		t.Fatalf("error = %v", err)
	}
}

func TestCandidateReportAcceptsValidFixture(t *testing.T) {
	t.Parallel()
	root, candidate := candidateFixture(t, []map[string]any{domainItem("Master Item")})
	before, err := hashing.Directory(root)
	if err != nil {
		t.Fatal(err)
	}
	report := CandidateReport(root, candidate, Options{})
	if err := report.Err(false); err != nil {
		t.Fatal(err)
	}
	after, err := hashing.Directory(root)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("validator mutated candidate: before %s, after %s", before, after)
	}
}

func TestReleaseReportRejectsFormattedManifest(t *testing.T) {
	t.Parallel()
	candidateRoot, candidate := candidateFixture(t, []map[string]any{domainItem("Master Item")})
	releaseRoot := filepath.Join(t.TempDir(), "release")
	release, err := manifest.Assemble(candidateRoot, releaseRoot, candidate, 1)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := json.MarshalIndent(release, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(releaseRoot, "manifest.json"), string(append(formatted, '\n')))

	report := ReleaseReport(releaseRoot, release, Options{})
	if issue := findIssue(report, "compact-json"); issue == nil || issue.File != "manifest.json" {
		t.Fatalf("issues = %#v, want compact manifest error", report.Issues)
	}
}

func TestCandidateValidationFailureModes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*testing.T, string, *manifest.Candidate)
		rule string
	}{
		{
			name: "truncated generated file",
			edit: func(t *testing.T, root string, _ *manifest.Candidate) {
				writeTestFile(t, filepath.Join(root, "example", "items.json"), "[")
			},
			rule: "valid-json",
		},
		{
			name: "hash mismatch",
			edit: func(t *testing.T, root string, _ *manifest.Candidate) {
				writeTestJSON(t, filepath.Join(root, "example", "items.json"), []map[string]any{domainItem("Master Item"), domainItem("Another Item")})
			},
			rule: "file-hash-agreement",
		},
		{
			name: "formatted generated file",
			edit: func(t *testing.T, root string, candidate *manifest.Candidate) {
				data, err := json.MarshalIndent([]map[string]any{domainItem("Master Item")}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(root, "example", "items.json"), string(append(data, '\n')))
				refreshCandidate(t, root, candidate)
			},
			rule: "compact-json",
		},
		{
			name: "missing master reference",
			edit: func(t *testing.T, root string, candidate *manifest.Candidate) {
				writeTestJSON(t, filepath.Join(root, "example", "items.json"), []map[string]any{domainItem("Missing Item")})
				refreshCandidate(t, root, candidate)
			},
			rule: "master-reference",
		},
		{
			name: "unexpected empty output",
			edit: func(t *testing.T, root string, candidate *manifest.Candidate) {
				writeTestJSON(t, filepath.Join(root, "example", "items.json"), []map[string]any{})
				refreshCandidate(t, root, candidate)
			},
			rule: "non-empty-dataset",
		},
		{
			name: "out of range minimum level",
			edit: func(t *testing.T, root string, candidate *manifest.Candidate) {
				item := domainItem("Master Item")
				item["minLevel"] = "101"
				writeTestJSON(t, filepath.Join(root, "example", "items.json"), []map[string]any{item})
				refreshCandidate(t, root, candidate)
			},
			rule: "valid-minimum-maximum",
		},
		{
			name: "temporary release file",
			edit: func(t *testing.T, root string, _ *manifest.Candidate) {
				writeTestFile(t, filepath.Join(root, "example", "items.json.partial"), "partial")
			},
			rule: "release-file-hygiene",
		},
		{
			name: "manifest disagreement",
			edit: func(_ *testing.T, _ string, candidate *manifest.Candidate) {
				candidate.GeneratedFiles = candidate.GeneratedFiles[1:]
			},
			rule: "manifest-file-agreement",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, candidate := candidateFixture(t, []map[string]any{domainItem("Master Item")})
			test.edit(t, root, &candidate)
			report := CandidateReport(root, candidate, Options{})
			issue := findIssue(report, test.rule)
			if issue == nil {
				t.Fatalf("issues = %#v, want rule %q", report.Issues, test.rule)
			}
			if issue.Severity != Error || issue.Dataset == "" || issue.File == "" || issue.RecordID == "" || issue.Message == "" {
				t.Fatalf("incomplete structured issue = %#v", issue)
			}
		})
	}
}

func TestDuplicateSetItemsAndEssenceReferencesAreRejected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setPath := filepath.Join(root, "gear-planner", "setBonusIndex.json")
	writeTestJSON(t, setPath, map[string]any{
		"Fixture Set": []map[string]any{{"name": "One", "minLevel": 1}, {"name": "One", "minLevel": 1}},
	})
	setFile := contracts.GeneratedFileMetadata{Domain: "gear-planner", Path: "gear-planner/setBonusIndex.json"}
	report := validateSetBonusIndex(setPath, setFile, masterIDs{})
	if findIssue(report, "duplicate-item") == nil {
		t.Fatalf("set issues = %#v", report.Issues)
	}

	writeTestJSON(t, filepath.Join(root, "essence-crafting", "effects.json"), []map[string]any{{"id": "known", "name": "Known"}})
	writeTestJSON(t, filepath.Join(root, "essence-crafting", "planner_entries.json"), []map[string]any{{
		"id": "entry", "sourceType": "unknown", "effectId": "missing", "enchantmentName": "Missing", "slotId": "ring", "affixType": "invalid",
	}})
	files := []contracts.GeneratedFileMetadata{
		{Domain: "essence-crafting", Path: "essence-crafting/effects.json"},
		{Domain: "essence-crafting", Path: "essence-crafting/planner_entries.json"},
	}
	report = validateRecordFiles(root, files, nil)
	if findIssue(report, "referential-integrity") == nil || findIssue(report, "valid-enum") == nil {
		t.Fatalf("essence issues = %#v", report.Issues)
	}
}

func TestMasterRejectsDuplicateIdentifiersAndMalformedRecords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeMaster(t, root, []map[string]any{
		masterItem("Duplicate"),
		masterItem("Duplicate"),
		{"pageTitle": "Missing fields"},
	})
	report := MasterReport(root)
	for _, rule := range []string{"unique-identifier", "required-fields"} {
		if findIssue(report, rule) == nil {
			t.Fatalf("issues = %#v, want %s", report.Issues, rule)
		}
	}
}

func TestMasterRejectsDuplicateJSONObjectKeys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, dataset.MasterIndexName), "{\"schemaVersion\":1,\"schemaVersion\":1,\"files\":[]}\n")
	report := MasterReport(root)
	if findIssue(report, "unique-identifier") == nil {
		t.Fatalf("issues = %#v", report.Issues)
	}
}

func TestCountDriftUsesRelativeBaselines(t *testing.T) {
	t.Parallel()
	baselineRoot := t.TempDir()
	currentRoot := t.TempDir()
	path := filepath.Join("example", "items.json")
	writeNRecords(t, filepath.Join(baselineRoot, path), 20)
	writeNRecords(t, filepath.Join(currentRoot, path), 5)
	baseline := manifest.Candidate{GeneratedFiles: []contracts.GeneratedFileMetadata{{Domain: "example", Path: filepath.ToSlash(path)}}}
	files := []contracts.GeneratedFileMetadata{{Domain: "example", Path: filepath.ToSlash(path)}}
	report := validateCountDrift(currentRoot, files, Options{BaselineRoot: baselineRoot, Baseline: &baseline, MaximumReduction: 0.5, MaximumIncrease: 1})
	if issue := findIssue(report, "record-count-reduction"); issue == nil || issue.Severity != Error {
		t.Fatalf("issues = %#v", report.Issues)
	}

	writeNRecords(t, filepath.Join(currentRoot, path), 50)
	report = validateCountDrift(currentRoot, files, Options{BaselineRoot: baselineRoot, Baseline: &baseline, MaximumReduction: 0.5, MaximumIncrease: 1})
	issue := findIssue(report, "record-count-increase")
	if issue == nil || issue.Severity != Warning || report.Err(false) != nil || report.Err(true) == nil {
		t.Fatalf("warning policy report = %#v", report)
	}
}

func candidateFixture(t *testing.T, domainItems []map[string]any) (string, manifest.Candidate) {
	t.Helper()
	root := t.TempDir()
	writeMaster(t, filepath.Join(root, "master"), []map[string]any{masterItem("Master Item"), masterItem("Another Item")})
	writeTestJSON(t, filepath.Join(root, "example", "items.json"), domainItems)
	masterHash, err := hashing.Directory(filepath.Join(root, "master"))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := manifest.BuildCandidate("81.3.0", strings.Repeat("a", 64), masterHash, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	return root, candidate
}

func refreshCandidate(t *testing.T, root string, candidate *manifest.Candidate) {
	t.Helper()
	masterHash, err := hashing.Directory(filepath.Join(root, "master"))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := manifest.BuildCandidate(candidate.GameVersion, candidate.SourceSHA256, masterHash, root, candidate.ManualPayloads)
	if err != nil {
		t.Fatal(err)
	}
	*candidate = updated
}

func writeMaster(t *testing.T, root string, records []map[string]any) {
	t.Helper()
	writeTestJSON(t, filepath.Join(root, "items.json"), records)
	writeTestJSON(t, filepath.Join(root, dataset.MasterIndexName), dataset.MasterIndex{
		SchemaVersion: 1,
		Files:         []dataset.MasterFile{{Category: "Test", Kind: "items", Path: "items.json"}},
	})
}

func masterItem(name string) map[string]any {
	return map[string]any{"pageTitle": name, "name": name, "type": "Trinket", "minLevel": "1", "enchantments": []any{}}
}

func domainItem(name string) map[string]any { return masterItem(name) }

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, string(append(data, '\n')))
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeNRecords(t *testing.T, path string, count int) {
	t.Helper()
	records := make([]map[string]any, count)
	for index := range records {
		records[index] = map[string]any{"id": index}
	}
	writeTestJSON(t, path, records)
}

func findIssue(report Report, rule string) *Issue {
	for index := range report.Issues {
		if report.Issues[index].Rule == rule {
			return &report.Issues[index]
		}
	}
	return nil
}
