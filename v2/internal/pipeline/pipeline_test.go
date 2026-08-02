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

	"yourddo-data-tools/v2/internal/config"
	"yourddo-data-tools/v2/internal/contracts"
	"yourddo-data-tools/v2/internal/publish"
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
			"Test Item": "{{Item|name=Test Item|type=Trinket|minlevel=1}}",
		}},
		Clock:  func() time.Time { return time.Unix(1, 0) },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	first, err := Execute(context.Background(), cfg, ExecuteOptions{DryRun: true}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	client := &pipelineS3Client{objects: activeS3Objects(first.Candidate.MasterDatasetSHA256)}
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

func activeS3Objects(masterHash string) map[string]string {
	return map[string]string{
		"latest.json":                     `{"gameVersion":"81.3.0","dataVersion":1,"baseUrl":"/releases/81.3.0/1"}`,
		"releases/81.3.0/1/manifest.json": `{"schemaVersion":1,"gameVersion":"81.3.0","dataVersion":1,"masterDatasetSha256":"` + masterHash + `","domains":[],"generatedFiles":[]}`,
	}
}

type failingSource struct{ err error }

func (f failingSource) FetchCategoryContent(context.Context, string) (map[string]string, error) {
	return nil, f.err
}

func (f failingSource) FetchPageContent(context.Context, string) (string, error) {
	return "", f.err
}
