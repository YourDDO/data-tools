package pipeline

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"yourddo-data-tools/internal/config"
	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/hashing"
	"yourddo-data-tools/internal/publish"
)

type fakeSource struct {
	pages map[string]string
}

func TestRunRejectsMalformedSourceBeforeCandidatePromotion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := testConfig(root, []string{"gear-planner"})
	_, err := Run(context.Background(), cfg, Dependencies{Source: fakeSource{pages: map[string]string{
		"Broken Item": "{{Item|name=Broken Item",
	}}})
	if err == nil {
		t.Fatal("Run succeeded with malformed source data")
	}
	if _, statErr := os.Stat(cfg.CandidateDir()); !os.IsNotExist(statErr) {
		t.Fatalf("candidate was promoted after malformed source: %v", statErr)
	}
}

func TestRunRejectsUnexpectedEmptyDomainOutput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := testConfig(root, []string{"zhentarim-attuned"})
	_, err := Run(context.Background(), cfg, Dependencies{Source: fakeSource{pages: map[string]string{
		"Ordinary Item": "{{Item|name=Ordinary Item|type=Trinket|minlevel=1}}",
	}}})
	if err == nil || !strings.Contains(err.Error(), "non-empty-dataset") {
		t.Fatalf("error = %v, want empty dataset validation failure", err)
	}
	if _, statErr := os.Stat(cfg.CandidateDir()); !os.IsNotExist(statErr) {
		t.Fatalf("candidate was promoted after empty output: %v", statErr)
	}
}

func testConfig(root string, domains []string) config.Config {
	cfg := config.Defaults()
	cfg.OutputDir = filepath.Join(root, "work")
	cfg.CompendiumBaseURL = "http://private.example"
	cfg.GameVersion = "81.3.0"
	cfg.Categories = []string{"Test"}
	cfg.Domains = domains
	return cfg
}

func (f fakeSource) FetchCategoryContent(context.Context, string) (map[string]string, error) {
	copy := make(map[string]string, len(f.pages))
	for key, value := range f.pages {
		copy[key] = value
	}
	return copy, nil
}

func (f fakeSource) FetchPageContent(context.Context, string) (string, error) {
	return "", nil
}

func TestRunBuildsCandidateThenDetectsUnchangedSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.OutputDir = filepath.Join(root, "work")
	cfg.CompendiumBaseURL = "http://private.example"
	cfg.GameVersion = "81.3.0"
	cfg.Categories = []string{"Test"}
	cfg.Domains = []string{"gear-planner", "zhentarim-attuned"}
	dependencies := Dependencies{
		Source: fakeSource{pages: map[string]string{
			"Test Item":            "{{Item|name=Test Item|type=Trinket|minlevel=1|enchantments={{ZhentarimAttuned}}}}",
			"Test Item (Upgraded)": "{{Item|name=Test Item|type=Trinket|minlevel=2|enchantments={{Ability|Strength|2}}}}",
		}},
	}
	first, err := Run(context.Background(), cfg, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Changed || first.Candidate.GameVersion != "81.3.0" {
		t.Fatalf("first result = %#v", first)
	}
	second, err := Run(context.Background(), cfg, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed || second.Candidate.SourceSHA256 != first.Candidate.SourceSHA256 {
		t.Fatalf("second result = %#v", second)
	}
	dependencies.Source = fakeSource{pages: map[string]string{
		"Test Item":            "{{Item|name=Test Item|type=Trinket|minlevel=3|enchantments={{ZhentarimAttuned}}}}",
		"Test Item (Upgraded)": "{{Item|name=Test Item|type=Trinket|minlevel=2|enchantments={{Ability|Strength|2}}}}",
	}}
	third, err := Run(context.Background(), cfg, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if !third.Changed || third.Candidate.SourceSHA256 == first.Candidate.SourceSHA256 {
		t.Fatalf("third result after input change = %#v", third)
	}
}

func TestExecutePreservesPrimaryFailureWhenCleanupFails(t *testing.T) {
	t.Parallel()
	primary := errors.New("source unavailable")
	cleanup := errors.New("cleanup unavailable")
	cfg := testConfig(t.TempDir(), []string{"gear-planner"})
	result, err := Execute(context.Background(), cfg, ExecuteOptions{DryRun: true}, OrchestratorDependencies{
		Source: failingSource{err: primary}, Clock: func() time.Time { return time.Unix(1, 0) },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RemoveAll: func(string) error { return cleanup },
	})
	if !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("error = %v, want both original and cleanup failures", err)
	}
	if result.Outcome != contracts.PipelineOutcomeFailed {
		t.Fatalf("outcome = %q", result.Outcome)
	}
}

func TestExecuteDoesNotCreateDataVersionForUnchangedMaster(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t.TempDir(), []string{"gear-planner"})
	dependencies := OrchestratorDependencies{
		Source: fakeSource{pages: map[string]string{
			"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1|enchantments={{ItemSet|Missing Runtime Set}}}}",
		}},
		Clock:  func() time.Time { return time.Unix(1, 0) },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	first, err := Execute(context.Background(), cfg, ExecuteOptions{DryRun: true}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	client := &pipelineS3Client{objects: activeS3Objects(first.Candidate.MasterDatasetSHA256, first.Candidate.ReleaseFingerprint)}
	store, err := publish.NewS3Store(client, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	clockCalls := 0
	dependencies.Active = store
	dependencies.Store = store
	dependencies.Clock = func() time.Time {
		clockCalls++
		return time.Unix(2, 0)
	}
	var logs bytes.Buffer
	dependencies.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	cfg.Domains = []string{"must-not-be-resolved"}
	second, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if second.Outcome != contracts.PipelineOutcomeNoChange || clockCalls != 0 || second.Release != nil || len(client.puts) != 0 {
		t.Fatalf("result = %#v, clock calls = %d, PutObject calls = %d", second, clockCalls, len(client.puts))
	}
	for _, stage := range second.Stages {
		if stage.Name == StageGenerateDomains || stage.Name == StageAssembleRelease {
			t.Fatalf("unchanged pipeline executed stage %s", stage.Name)
		}
	}
	for _, field := range []string{
		`"msg":"item-set definitions are missing from the manual payload"`,
		`"source":"setBonusIndex"`,
		`"missing_sets":["Missing Runtime Set"]`,
		`"latestObjectKey":"latest.json"`,
		`"activeManifestKey":"releases/81.3.0/1/manifest.json"`,
		`"activeMasterHash":"` + first.Candidate.MasterDatasetSHA256 + `"`,
		`"generatedMasterHash":"` + first.Candidate.MasterDatasetSHA256 + `"`,
		`"comparisonResult":"no-change"`,
	} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("comparison logs do not contain %s:\n%s", field, logs.String())
		}
	}
}

func TestExecuteTreatsPreviousOutputContractAsChanged(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t.TempDir(), []string{"gear-planner"})
	dependencies := OrchestratorDependencies{
		Source: fakeSource{pages: map[string]string{
			"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}",
		}},
		Clock:  func() time.Time { return time.Unix(1, 0) },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	current, err := Execute(context.Background(), cfg, ExecuteOptions{DryRun: true}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	previousFingerprint := hashing.Combine(current.Candidate.MasterDatasetSHA256)
	client := &pipelineS3Client{objects: activeS3Objects(current.Candidate.MasterDatasetSHA256, previousFingerprint)}
	store, err := publish.NewS3Store(client, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	dependencies.Active = store
	dependencies.Store = store
	dependencies.Clock = func() time.Time { return time.Unix(2, 0) }

	result, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != contracts.PipelineOutcomePublished || !result.Changed {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecuteManualPayloadChangeDetection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := testConfig(root, []string{"gear-planner"})
	cfg.ManualInputDir = filepath.Join(root, "inputs", "manual")
	writeManualPayload(t, cfg.ManualInputDir, "settings.json", `{"value":1,"ordered":[2,1]}`)
	dependencies := OrchestratorDependencies{
		Source: fakeSource{pages: map[string]string{
			"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}",
		}},
		Clock:  func() time.Time { return time.Unix(1, 0) },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	first, err := Execute(context.Background(), cfg, ExecuteOptions{DryRun: true}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Candidate.ManualPayloads) != 1 {
		t.Fatalf("manual payloads = %#v", first.Candidate.ManualPayloads)
	}

	writeManualPayload(t, cfg.ManualInputDir, "settings.json", "{\n  \"ordered\": [2, 1],\n  \"value\": 1\n}\n")
	unchangedClient := &pipelineS3Client{objects: activeS3Objects(first.Candidate.MasterDatasetSHA256, first.Candidate.ReleaseFingerprint)}
	unchangedStore, err := publish.NewS3Store(unchangedClient, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	clockCalls := 0
	formattedConfig := cfg
	formattedConfig.Domains = []string{"must-not-be-resolved"}
	formatted, err := Execute(context.Background(), formattedConfig, ExecuteOptions{Publish: true}, OrchestratorDependencies{
		Source: dependencies.Source, Active: unchangedStore, Store: unchangedStore,
		Clock:  func() time.Time { clockCalls++; return time.Unix(2, 0) },
		Logger: dependencies.Logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	if formatted.Outcome != contracts.PipelineOutcomeNoChange || clockCalls != 0 || len(unchangedClient.puts) != 0 {
		t.Fatalf("formatted result = %#v, clock calls = %d, writes = %d", formatted, clockCalls, len(unchangedClient.puts))
	}

	writeManualPayload(t, cfg.ManualInputDir, "settings.json", `{"value":2,"ordered":[2,1]}`)
	changedClient := &pipelineS3Client{objects: activeS3Objects(first.Candidate.MasterDatasetSHA256, first.Candidate.ReleaseFingerprint)}
	changedStore, err := publish.NewS3Store(changedClient, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, OrchestratorDependencies{
		Source: dependencies.Source, Active: changedStore, Store: changedStore,
		Clock: func() time.Time { return time.Unix(2, 0) }, Logger: dependencies.Logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Outcome != contracts.PipelineOutcomePublished || changed.Candidate.MasterDatasetSHA256 != first.Candidate.MasterDatasetSHA256 || changed.Candidate.ReleaseFingerprint == first.Candidate.ReleaseFingerprint {
		t.Fatalf("changed result = %#v", changed)
	}
	wantManualKey := "releases/81.3.0/2/manual/settings.json"
	foundManual := false
	for _, input := range changedClient.puts {
		if aws.ToString(input.Key) == wantManualKey {
			foundManual = true
		}
	}
	if !foundManual {
		t.Fatalf("manual object %s was not uploaded", wantManualKey)
	}
}

func TestExecuteAddingOrRemovingManualPayloadCreatesRelease(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"add", "remove"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			cfg := testConfig(root, []string{"gear-planner"})
			cfg.ManualInputDir = filepath.Join(root, "inputs", "manual")
			writeManualPayload(t, cfg.ManualInputDir, "first.json", `{"value":1}`)
			dependencies := OrchestratorDependencies{
				Source: fakeSource{pages: map[string]string{"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}"}},
				Clock:  func() time.Time { return time.Unix(1, 0) }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}
			first, err := Execute(context.Background(), cfg, ExecuteOptions{DryRun: true}, dependencies)
			if err != nil {
				t.Fatal(err)
			}
			if mutation == "add" {
				writeManualPayload(t, cfg.ManualInputDir, "nested/second.json", `{"value":2}`)
			} else if err := os.Remove(filepath.Join(cfg.ManualInputDir, "first.json")); err != nil {
				t.Fatal(err)
			}
			client := &pipelineS3Client{objects: activeS3Objects(first.Candidate.MasterDatasetSHA256, first.Candidate.ReleaseFingerprint)}
			store, err := publish.NewS3Store(client, "yourddo-data-prod")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, OrchestratorDependencies{
				Source: dependencies.Source, Active: store, Store: store,
				Clock: func() time.Time { return time.Unix(2, 0) }, Logger: dependencies.Logger,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != contracts.PipelineOutcomePublished || result.Candidate.ReleaseFingerprint == first.Candidate.ReleaseFingerprint {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestExecuteRejectsMalformedManualJSONBeforePublication(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := testConfig(root, []string{"gear-planner"})
	cfg.ManualInputDir = filepath.Join(root, "inputs", "manual")
	writeManualPayload(t, cfg.ManualInputDir, "broken.json", `{"value":`)
	client := &pipelineS3Client{objects: map[string]string{}}
	store, err := publish.NewS3Store(client, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	clockCalls := 0
	result, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, OrchestratorDependencies{
		Source: fakeSource{pages: map[string]string{"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}"}},
		Active: store, Store: store, Clock: func() time.Time { clockCalls++; return time.Unix(2, 0) },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil || !strings.Contains(err.Error(), "pipeline stage prepare-manual failed") {
		t.Fatalf("error = %v", err)
	}
	if result.Outcome != contracts.PipelineOutcomeFailed || clockCalls != 0 || len(client.puts) != 0 {
		t.Fatalf("result = %#v, clock calls = %d, writes = %d", result, clockCalls, len(client.puts))
	}
}

func TestExecuteTreatsLegacyManifestWithoutFingerprintAsChanged(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t.TempDir(), []string{"gear-planner"})
	client := &pipelineS3Client{objects: map[string]string{
		"latest.json":                     `{"gameVersion":"81.3.0","dataVersion":1,"baseUrl":"/releases/81.3.0/1"}`,
		"releases/81.3.0/1/manifest.json": `{"schemaVersion":1,"gameVersion":"81.3.0","dataVersion":1,"masterDatasetSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","domains":[],"generatedFiles":[]}`,
	}}
	store, err := publish.NewS3Store(client, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, OrchestratorDependencies{
		Source: fakeSource{pages: map[string]string{"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}"}},
		Active: store, Store: store, Clock: func() time.Time { return time.Unix(2, 0) },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != contracts.PipelineOutcomePublished || len(client.puts) == 0 {
		t.Fatalf("result = %#v, writes = %d", result, len(client.puts))
	}
}

func TestExecutePublishesChangedAndInitialMasterToS3(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		objects map[string]string
	}{
		{name: "changed", objects: activeS3Objects(strings.Repeat("a", 64))},
		{name: "initial publication", objects: map[string]string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := testConfig(t.TempDir(), []string{"gear-planner"})
			client := &pipelineS3Client{objects: test.objects}
			store, err := publish.NewS3Store(client, "yourddo-data-prod")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, OrchestratorDependencies{
				Source: fakeSource{pages: map[string]string{
					"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}",
				}},
				Active: store,
				Store:  store,
				Clock:  func() time.Time { return time.Unix(2, 0) },
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != contracts.PipelineOutcomePublished || !result.Published || !result.Changed {
				t.Fatalf("result = %#v", result)
			}
			if len(client.puts) == 0 || aws.ToString(client.puts[len(client.puts)-1].Key) != "latest.json" {
				t.Fatalf("PutObject calls = %#v", client.puts)
			}
		})
	}
}

func TestExecuteFailsActiveS3LookupWithoutPublishing(t *testing.T) {
	t.Parallel()
	const latest = `{"gameVersion":"81.3.0","dataVersion":1,"baseUrl":"/releases/81.3.0/1"}`
	tests := []struct {
		name      string
		objects   map[string]string
		getErrors map[string]error
	}{
		{name: "malformed latest", objects: map[string]string{"latest.json": "{"}},
		{name: "missing manifest", objects: map[string]string{"latest.json": latest}},
		{name: "malformed manifest", objects: map[string]string{"latest.json": latest, "releases/81.3.0/1/manifest.json": "{"}},
		{name: "permission error", getErrors: map[string]error{"latest.json": &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied", Fault: smithy.FaultClient}}},
		{name: "network error", getErrors: map[string]error{"latest.json": errors.New("connection reset")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := testConfig(t.TempDir(), []string{"must-not-be-resolved"})
			client := &pipelineS3Client{objects: test.objects, getErrors: test.getErrors}
			store, err := publish.NewS3Store(client, "yourddo-data-prod")
			if err != nil {
				t.Fatal(err)
			}
			clockCalls := 0
			result, err := Execute(context.Background(), cfg, ExecuteOptions{Publish: true}, OrchestratorDependencies{
				Source: fakeSource{pages: map[string]string{
					"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}",
				}},
				Active: store,
				Store:  store,
				Clock: func() time.Time {
					clockCalls++
					return time.Unix(2, 0)
				},
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			})
			if err == nil || !strings.Contains(err.Error(), "pipeline stage compare failed") {
				t.Fatalf("error = %v", err)
			}
			if result.Outcome != contracts.PipelineOutcomeFailed || len(client.puts) != 0 || clockCalls != 0 {
				t.Fatalf("result = %#v, PutObject calls = %d, clock calls = %d", result, len(client.puts), clockCalls)
			}
		})
	}
}

type pipelineS3Client struct {
	objects   map[string]string
	getErrors map[string]error
	puts      []*s3.PutObjectInput
}

func (c *pipelineS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := aws.ToString(input.Key)
	if err := c.getErrors[key]; err != nil {
		return nil, err
	}
	value, exists := c.objects[key]
	if !exists {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(value))}, nil
}

func (c *pipelineS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	copy := *input
	c.puts = append(c.puts, &copy)
	return &s3.PutObjectOutput{}, nil
}

func activeS3Objects(masterHash string, releaseFingerprints ...string) map[string]string {
	releaseFingerprint := masterHash
	if len(releaseFingerprints) != 0 {
		releaseFingerprint = releaseFingerprints[0]
	}
	return map[string]string{
		"latest.json":                     `{"gameVersion":"81.3.0","dataVersion":1,"baseUrl":"/releases/81.3.0/1"}`,
		"releases/81.3.0/1/manifest.json": `{"schemaVersion":2,"gameVersion":"81.3.0","dataVersion":1,"masterDatasetSha256":"` + masterHash + `","releaseFingerprint":"` + releaseFingerprint + `","manualPayloads":[],"domains":[],"generatedFiles":[]}`,
	}
}

type failingSource struct{ err error }

func writeManualPayload(t *testing.T, root, relative, data string) {
	t.Helper()
	filePath := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f failingSource) FetchCategoryContent(context.Context, string) (map[string]string, error) {
	return nil, f.err
}

func (f failingSource) FetchPageContent(context.Context, string) (string, error) {
	return "", f.err
}
