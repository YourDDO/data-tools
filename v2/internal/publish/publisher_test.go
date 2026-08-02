package publish

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yourddo-data-tools/v2/internal/hashing"
	"yourddo-data-tools/v2/internal/manifest"
)

type recordingStore struct {
	keys    []string
	options []PutOptions
	values  map[string][]byte
	failKey string
}

func (s *recordingStore) Put(_ context.Context, key string, data []byte, options PutOptions) error {
	s.keys = append(s.keys, key)
	s.options = append(s.options, options)
	if key == s.failKey {
		return errors.New("injected failure")
	}
	if s.values == nil {
		s.values = make(map[string][]byte)
	}
	s.values[key] = append([]byte(nil), data...)
	return nil
}

func TestPublisherAssignsDataVersionAndUpdatesLatestLast(t *testing.T) {
	t.Parallel()
	root, candidate := testCandidate(t)
	store := &recordingStore{}
	publisher, err := New(store, func() time.Time { return time.Unix(1785175200, 0) })
	if err != nil {
		t.Fatal(err)
	}
	release, err := publisher.Publish(context.Background(), root, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if release.DataVersion != 1785175200 {
		t.Fatalf("dataVersion = %d", release.DataVersion)
	}
	want := []string{
		"releases/81.3.0/1785175200/master/items.json",
		"releases/81.3.0/1785175200/master/master-index.json",
		"releases/81.3.0/1785175200/manifest.json",
		"latest.json",
	}
	if len(store.keys) != len(want) {
		t.Fatalf("keys = %#v", store.keys)
	}
	for index := range want {
		if store.keys[index] != want[index] {
			t.Fatalf("keys = %#v", store.keys)
		}
		if store.options[index].Immutable != (store.keys[index] != "latest.json") {
			t.Fatalf("Put options for %s = %#v", store.keys[index], store.options[index])
		}
	}
	var latest manifest.Latest
	if err := json.Unmarshal(store.values["latest.json"], &latest); err != nil {
		t.Fatal(err)
	}
	if latest.BaseURL != "/releases/81.3.0/1785175200" {
		t.Fatalf("latest = %#v", latest)
	}
}

func TestPublisherDoesNotUpdateLatestAfterFailure(t *testing.T) {
	t.Parallel()
	for _, failKey := range []string{
		"releases/81.3.0/1/master/items.json",
		"releases/81.3.0/1/manifest.json",
	} {
		failKey := failKey
		t.Run(failKey, func(t *testing.T) {
			t.Parallel()
			root, candidate := testCandidate(t)
			store := &recordingStore{failKey: failKey}
			publisher, _ := New(store, func() time.Time { return time.Unix(1, 0) })
			if _, err := publisher.Publish(context.Background(), root, candidate); err == nil {
				t.Fatal("Publish succeeded, want failure")
			}
			for _, key := range store.keys {
				if key == "latest.json" {
					t.Fatal("latest pointer was updated after a failed upload")
				}
			}
		})
	}
}

func TestPublisherDoesNotCallStoreWhenValidationFails(t *testing.T) {
	t.Parallel()
	root, candidate := testCandidate(t)
	if err := os.WriteFile(filepath.Join(root, "master", "items.json"), []byte("["), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &recordingStore{}
	publisher, _ := New(store, func() time.Time { return time.Unix(1, 0) })
	if _, err := publisher.Publish(context.Background(), root, candidate); err == nil {
		t.Fatal("Publish succeeded with malformed generated data")
	}
	if len(store.keys) != 0 {
		t.Fatalf("publication store was called before validation completed: %v", store.keys)
	}
}

func TestLocalStoreEnforcesImmutability(t *testing.T) {
	t.Parallel()
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "releases/one/data.json", []byte("one"), PutOptions{Immutable: true}); err != nil {
		t.Fatal(err)
	}
	err = store.Put(context.Background(), "releases/one/data.json", []byte("two"), PutOptions{Immutable: true})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
}

func TestPublisherWritesLocalReleaseAndLatest(t *testing.T) {
	t.Parallel()
	sourceRoot, candidate := testCandidate(t)
	destination := t.TempDir()
	store, err := NewLocalStore(destination)
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := New(store, func() time.Time { return time.Unix(10, 0) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.Publish(context.Background(), sourceRoot, candidate); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(destination, "releases", "81.3.0", "10", "master", "items.json"),
		filepath.Join(destination, "releases", "81.3.0", "10", "manifest.json"),
		filepath.Join(destination, "latest.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("published path %s: %v", path, err)
		}
	}
}

func testCandidate(t *testing.T) (string, manifest.Candidate) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "master", "items.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[{\"pageTitle\":\"Test Item\",\"name\":\"Test Item\",\"type\":\"Trinket\",\"minLevel\":\"1\",\"enchantments\":[]}]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(root, "master", "master-index.json")
	if err := os.WriteFile(indexPath, []byte("{\"schemaVersion\":1,\"files\":[{\"category\":\"Test\",\"kind\":\"items\",\"path\":\"items.json\"}]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	masterHash, err := hashing.Directory(filepath.Join(root, "master"))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := manifest.BuildCandidate("81.3.0", "source", masterHash, root)
	if err != nil {
		t.Fatal(err)
	}
	return root, candidate
}
