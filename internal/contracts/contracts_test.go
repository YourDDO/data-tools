package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReleaseManifestJSONContract(t *testing.T) {
	t.Parallel()
	value := ReleaseManifest{
		SchemaVersion:       ReleaseManifestSchemaVersion,
		ReleaseIdentity:     ReleaseIdentity{GameVersion: "81.3.0", DataVersion: 1785175200},
		MasterDatasetSHA256: "master-hash",
		ReleaseFingerprint:  "release-fingerprint",
		ManualPayloads: []ManualPayloadMetadata{{
			Name: "example", Path: "manual/example.json", SizeBytes: 18, SHA256: "manual-hash",
		}},
		Domains: []DatasetMetadata{{
			Domain: "gear-planner", FileCount: 1, SizeBytes: 42, SHA256: "domain-hash",
		}},
		GeneratedFiles: []GeneratedFileMetadata{{
			Domain: "gear-planner", Path: "gear-planner/items.json", SizeBytes: 42, SHA256: "file-hash",
		}},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schemaVersion":2,"gameVersion":"81.3.0","dataVersion":1785175200,"masterDatasetSha256":"master-hash","releaseFingerprint":"release-fingerprint","manualPayloads":[{"name":"example","path":"manual/example.json","sizeBytes":18,"sha256":"manual-hash"}],"domains":[{"domain":"gear-planner","fileCount":1,"sizeBytes":42,"sha256":"domain-hash"}],"generatedFiles":[{"domain":"gear-planner","path":"gear-planner/items.json","sizeBytes":42,"sha256":"file-hash"}]}`
	if string(data) != want {
		t.Fatalf("manifest JSON = %s\nwant          = %s", data, want)
	}
}

func TestLatestJSONContract(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(Latest{
		ReleaseIdentity: ReleaseIdentity{GameVersion: "81.3.0", DataVersion: 1785175200},
		BaseURL:         "/releases/81.3.0/1785175200",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"gameVersion":"81.3.0","dataVersion":1785175200,"baseUrl":"/releases/81.3.0/1785175200"}`
	if string(data) != want {
		t.Fatalf("latest JSON = %s, want %s", data, want)
	}
}

func TestEssenceCraftingAugmentPlacementJSONContract(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(EssenceCraftingAugmentPlacement{
		ItemCategoryID: "armor",
		SlotTypeIDs:    []string{"colorless", "blue"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"augmentSlotTypeIds"`) {
		t.Fatalf("placement JSON = %s, missing augmentSlotTypeIds", data)
	}
	if strings.Contains(string(data), `"slotTypeIds"`) {
		t.Fatalf("placement JSON = %s, contains deprecated slotTypeIds", data)
	}
}

func TestPipelineResultJSONContract(t *testing.T) {
	t.Parallel()
	identity := ReleaseIdentity{GameVersion: "81.3.0", DataVersion: 1785175200}
	data, err := json.Marshal(PipelineResult{
		Outcome: PipelineOutcomePublished, Changed: true, Published: true, OutputDir: "build/output", Release: &identity,
		Stages: []PipelineStageResult{{Name: "publish", Status: "succeeded", Changed: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"outcome":"published","changed":true,"published":true,"outputDir":"build/output","release":{"gameVersion":"81.3.0","dataVersion":1785175200},"stages":[{"name":"publish","status":"succeeded","changed":true}]}`
	if string(data) != want {
		t.Fatalf("pipeline result JSON = %s, want %s", data, want)
	}
}
