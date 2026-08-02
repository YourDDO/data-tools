package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"yourddo-data-tools/internal/config"
	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/manifest"
)

func TestPipelineEndToEndPublishesExpectedReleaseAndThenDetectsNoChange(t *testing.T) {
	setSafeEnvironment(t)
	source := &fixtureSource{pages: readSourceFixture(t)}
	server := newCompendiumServer(t, source)
	defer server.Close()

	root := t.TempDir()
	workRoot := filepath.Join(root, "work")
	publishRoot := filepath.Join(root, "published")
	args := pipelineArgs(server.URL, workRoot, publishRoot, true, false)
	var stdout, stderr bytes.Buffer
	dependencies := commandDependencies{
		stdout: &stdout, stderr: &stderr,
		clock: func() time.Time { return time.Unix(1785175200, 0) },
	}
	if err := run(context.Background(), args, dependencies); err != nil {
		t.Fatalf("first pipeline run: %v\nlogs:\n%s", err, stderr.String())
	}
	result := decodePipelineResult(t, stdout.Bytes())
	if result.Outcome != contracts.PipelineOutcomePublished || !result.Published || !result.Changed {
		t.Fatalf("result = %#v", result)
	}
	assertStructuredLogs(t, stderr.String())

	releaseRoot := filepath.Join(publishRoot, "releases", "81.3.0", "1785175200")
	assertExpectedReleaseFiles(t, releaseRoot)
	release, err := decodeManifest(filepath.Join(releaseRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if release.MasterDatasetSHA256 == "" || len(release.GeneratedFiles) != 6 {
		t.Fatalf("release manifest = %#v", release)
	}
	latestBefore, err := os.ReadFile(filepath.Join(publishRoot, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := run(context.Background(), args, dependencies); err != nil {
		t.Fatalf("no-change pipeline run: %v\nlogs:\n%s", err, stderr.String())
	}
	result = decodePipelineResult(t, stdout.Bytes())
	if result.Outcome != contracts.PipelineOutcomeNoChange || result.Changed || result.Release != nil {
		t.Fatalf("no-change result = %#v", result)
	}
	latestAfter, err := os.ReadFile(filepath.Join(publishRoot, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(latestBefore, latestAfter) {
		t.Fatal("no-change run altered latest.json")
	}
	versions, err := os.ReadDir(filepath.Join(publishRoot, "releases", "81.3.0"))
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("no-change run created %d data versions", len(versions))
	}
}

func TestPipelineDryRunCleansWorkAndDoesNotPublish(t *testing.T) {
	setSafeEnvironment(t)
	source := &fixtureSource{pages: readSourceFixture(t)}
	server := newCompendiumServer(t, source)
	defer server.Close()
	root := t.TempDir()
	workRoot := filepath.Join(root, "work")
	publishRoot := filepath.Join(root, "published")
	var stdout bytes.Buffer
	err := run(context.Background(), pipelineArgs(server.URL, workRoot, publishRoot, true, true), commandDependencies{
		stdout: &stdout, stderr: &bytes.Buffer{}, clock: func() time.Time { return time.Unix(1785175200, 0) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result := decodePipelineResult(t, stdout.Bytes()); result.Outcome != contracts.PipelineOutcomeDryRun || !result.Changed || result.Published {
		t.Fatalf("dry-run result = %#v", result)
	}
	if _, err := os.Stat(publishRoot); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote publication root: %v", err)
	}
	entries, err := os.ReadDir(workRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary work was not cleaned: %v", entries)
	}
}

func TestPipelineDryRunWithoutPublishRootPreservesCandidate(t *testing.T) {
	setSafeEnvironment(t)
	source := &fixtureSource{pages: readSourceFixture(t)}
	server := newCompendiumServer(t, source)
	defer server.Close()
	workRoot := filepath.Join(t.TempDir(), "work")
	args := []string{
		"--base-url=" + server.URL, "--api-path=/api.php", "--game-version=81.3.0",
		"--output-dir=" + workRoot, "--categories=Test", "--domains=gear-planner,zhentarim-attuned",
		"--publish=false", "--dry-run", "--debug-preserve",
	}
	var stdout bytes.Buffer
	if err := run(context.Background(), args, commandDependencies{
		stdout: &stdout, stderr: &bytes.Buffer{}, clock: func() time.Time { return time.Unix(1785175200, 0) },
	}); err != nil {
		t.Fatal(err)
	}
	result := decodePipelineResult(t, stdout.Bytes())
	if result.Outcome != contracts.PipelineOutcomeDryRun || !result.Changed || result.Published {
		t.Fatalf("dry-run result = %#v", result)
	}
	candidateData, err := os.ReadFile(filepath.Join(result.OutputDir, "candidate", "candidate.json"))
	if err != nil {
		t.Fatalf("read preserved candidate: %v", err)
	}
	var candidate manifest.Candidate
	if err := json.Unmarshal(candidateData, &candidate); err != nil {
		t.Fatalf("decode preserved candidate: %v", err)
	}
	if candidate.GameVersion != "81.3.0" || candidate.MasterDatasetSHA256 == "" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestFailedValidationCannotAlterLatest(t *testing.T) {
	setSafeEnvironment(t)
	source := &fixtureSource{pages: readSourceFixture(t)}
	server := newCompendiumServer(t, source)
	defer server.Close()
	root := t.TempDir()
	workRoot := filepath.Join(root, "work")
	publishRoot := filepath.Join(root, "published")
	dependencies := commandDependencies{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, clock: func() time.Time { return time.Unix(100, 0) }}
	if err := run(context.Background(), pipelineArgs(server.URL, workRoot, publishRoot, true, false), dependencies); err != nil {
		t.Fatal(err)
	}
	latestPath := filepath.Join(publishRoot, "latest.json")
	latestBefore, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal(err)
	}

	source.set(map[string]string{"Ordinary Item": "{{Item|name=Ordinary Item|type=Trinket|minlevel=1}}"})
	var stdout bytes.Buffer
	dependencies.stdout = &stdout
	dependencies.clock = func() time.Time { return time.Unix(101, 0) }
	err = run(context.Background(), []string{
		"--base-url=" + server.URL, "--api-path=/api.php", "--game-version=81.3.0",
		"--output-dir=" + workRoot, "--categories=Test", "--domains=zhentarim-attuned",
		"--publish=true", "--publish-root=" + publishRoot,
	}, dependencies)
	if err == nil || !strings.Contains(err.Error(), "pipeline stage validate failed") {
		t.Fatalf("error = %v", err)
	}
	if result := decodePipelineResult(t, stdout.Bytes()); result.Outcome != contracts.PipelineOutcomeFailed || result.Published {
		t.Fatalf("failed result = %#v", result)
	}
	latestAfter, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(latestBefore, latestAfter) {
		t.Fatal("failed validation altered latest.json")
	}
}

func TestConfigurationFailureIsStructuredAndDoesNotLogCredentials(t *testing.T) {
	setSafeEnvironment(t)
	const secret = "do-not-log-this"
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{
		"--base-url=https://user:" + secret + "@example.com", "--api-path=/api.php", "--game-version=81.3.0",
	}, commandDependencies{stdout: &stdout, stderr: &stderr})
	if err == nil || !strings.Contains(err.Error(), "pipeline stage configuration failed") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(stderr.String(), secret) || strings.Contains(err.Error(), secret) {
		t.Fatalf("configuration failure exposed credentials: error=%v logs=%s", err, stderr.String())
	}
	if result := decodePipelineResult(t, stdout.Bytes()); result.Outcome != contracts.PipelineOutcomeFailed {
		t.Fatalf("result = %#v", result)
	}
	assertStructuredLogs(t, stderr.String())
}

func pipelineArgs(baseURL, workRoot, publishRoot string, publishEnabled, dryRun bool) []string {
	return []string{
		"--base-url=" + baseURL, "--api-path=/api.php", "--game-version=81.3.0",
		"--output-dir=" + workRoot, "--categories=Test", "--domains=gear-planner,zhentarim-attuned",
		fmt.Sprintf("--publish=%t", publishEnabled), fmt.Sprintf("--dry-run=%t", dryRun), "--publish-root=" + publishRoot,
	}
}

func setSafeEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		config.CompendiumBaseURLEnv, config.CompendiumAPIPathEnv, config.OutputDirEnv,
		config.AWSRegionEnv, config.DataBucketEnv, config.CDNBaseURLEnv, config.GameVersionEnv,
		config.PublishEnabledEnv, config.WarningsAsErrorsEnv,
	} {
		t.Setenv(name, "")
	}
	t.Setenv(config.PublishEnabledEnv, "false")
	t.Setenv(config.WarningsAsErrorsEnv, "false")
	t.Setenv(config.ManualInputDirEnv, filepath.Join(t.TempDir(), "manual"))
}

func readSourceFixture(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pages map[string]string
	if err := json.Unmarshal(data, &pages); err != nil {
		t.Fatal(err)
	}
	return pages
}

type fixtureSource struct {
	mu    sync.RWMutex
	pages map[string]string
}

func (s *fixtureSource) set(pages map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages = pages
}

func (s *fixtureSource) snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string, len(s.pages))
	for title, content := range s.pages {
		result[title] = content
	}
	return result
}

func newCompendiumServer(t *testing.T, source *fixtureSource) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api.php" || request.URL.Query().Get("gcmtitle") != "Category:Test" {
			t.Errorf("unexpected Compendium request %s with query keys %v", request.URL.Path, request.URL.Query())
			http.Error(writer, "unexpected request", http.StatusBadRequest)
			return
		}
		type revision struct {
			Slots struct {
				Main struct {
					Content string `json:"content"`
				} `json:"main"`
			} `json:"slots"`
		}
		type page struct {
			Title     string     `json:"title"`
			Revisions []revision `json:"revisions"`
		}
		response := struct {
			Query struct {
				Pages []page `json:"pages"`
			} `json:"query"`
		}{}
		for title, content := range source.snapshot() {
			value := page{Title: title, Revisions: []revision{{}}}
			value.Revisions[0].Slots.Main.Content = content
			response.Query.Pages = append(response.Query.Pages, value)
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}

func decodePipelineResult(t *testing.T, data []byte) contracts.PipelineResult {
	t.Helper()
	var result contracts.PipelineResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode pipeline result %q: %v", data, err)
	}
	return result
}

func decodeManifest(path string) (manifest.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.Manifest{}, err
	}
	var value manifest.Manifest
	if err := json.Unmarshal(data, &value); err != nil {
		return manifest.Manifest{}, err
	}
	return value, nil
}

func assertExpectedReleaseFiles(t *testing.T, releaseRoot string) {
	t.Helper()
	expectedRoot := filepath.Join("testdata", "expected")
	err := filepath.WalkDir(expectedRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(expectedRoot, path)
		if err != nil {
			return err
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(releaseRoot, relative))
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("release file %s does not match fixture\ngot:  %s\nwant: %s", relative, got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertStructuredLogs(t *testing.T, logs string) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var value map[string]any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			t.Fatalf("log line %d is not structured JSON: %q: %v", lineNumber+1, line, err)
		}
		if value["stage"] == nil {
			t.Fatalf("log line %d has no stage: %s", lineNumber+1, line)
		}
	}
}
