package pipeline

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yourddo-data-tools/v2/internal/config"
	"yourddo-data-tools/v2/internal/contracts"
)

type activeHash string

func (h activeHash) ActiveMasterHash(context.Context) (string, bool, error) {
	return string(h), true, nil
}

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
	clockCalls := 0
	dependencies.Active = activeHash(first.Candidate.MasterDatasetSHA256)
	dependencies.Clock = func() time.Time {
		clockCalls++
		return time.Unix(2, 0)
	}
	second, err := Execute(context.Background(), cfg, ExecuteOptions{DryRun: true}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if second.Outcome != contracts.PipelineOutcomeNoChange || clockCalls != 0 || second.Release != nil {
		t.Fatalf("result = %#v, clock calls = %d", second, clockCalls)
	}
	for _, stage := range second.Stages {
		if stage.Name == StageGenerateDomains || stage.Name == StageAssembleRelease {
			t.Fatalf("unchanged pipeline executed stage %s", stage.Name)
		}
	}
}

type failingSource struct{ err error }

func (f failingSource) FetchCategoryContent(context.Context, string) (map[string]string, error) {
	return nil, f.err
}

func (f failingSource) FetchPageContent(context.Context, string) (string, error) {
	return "", f.err
}
