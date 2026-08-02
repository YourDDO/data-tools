package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"yourddo-data-tools/v2/internal/manifest"
	"yourddo-data-tools/v2/internal/validation"
)

type Publisher struct {
	store ObjectStore
	clock func() time.Time
}

func New(store ObjectStore, clock func() time.Time) (*Publisher, error) {
	if store == nil || clock == nil {
		return nil, fmt.Errorf("publication store and clock are required")
	}
	return &Publisher{store: store, clock: clock}, nil
}

func (p *Publisher) Publish(ctx context.Context, root string, candidate manifest.Candidate) (manifest.Manifest, error) {
	if candidate.GameVersion == "" || candidate.GameVersion != path.Base(candidate.GameVersion) || strings.ContainsAny(candidate.GameVersion, `/\`) {
		return manifest.Manifest{}, fmt.Errorf("invalid game version in candidate metadata")
	}
	if err := validation.Candidate(root, candidate); err != nil {
		return manifest.Manifest{}, fmt.Errorf("validate candidate before publication: %w", err)
	}
	release, err := manifest.Release(candidate, p.clock().UTC().Unix())
	if err != nil {
		return manifest.Manifest{}, err
	}
	if err := p.uploadGeneratedFiles(ctx, root, release); err != nil {
		return manifest.Manifest{}, err
	}
	if err := p.publishManifest(ctx, release); err != nil {
		return manifest.Manifest{}, err
	}
	if err := p.Activate(ctx, release); err != nil {
		return manifest.Manifest{}, err
	}
	return release, nil
}

// Upload publishes every immutable object in an already assembled and
// validated release. It deliberately does not update latest.json.
func (p *Publisher) Upload(ctx context.Context, root string, release manifest.Manifest) error {
	if err := validation.Release(root, release); err != nil {
		return fmt.Errorf("validate assembled release before publication: %w", err)
	}
	if err := p.uploadGeneratedFiles(ctx, root, release); err != nil {
		return err
	}
	manifestData, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read assembled release manifest: %w", err)
	}
	manifestKey := path.Join(releaseBaseKey(release), "manifest.json")
	if err := p.store.Put(ctx, manifestKey, manifestData, PutOptions{Immutable: true}); err != nil {
		return fmt.Errorf("publish release manifest: %w", err)
	}
	return nil
}

func (p *Publisher) uploadGeneratedFiles(ctx context.Context, root string, release manifest.Manifest) error {
	baseKey := releaseBaseKey(release)
	for _, file := range release.GeneratedFiles {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			return fmt.Errorf("read release file %s: %w", file.Path, err)
		}
		key := path.Join(baseKey, file.Path)
		if err := p.store.Put(ctx, key, data, PutOptions{Immutable: true}); err != nil {
			return fmt.Errorf("publish release file %s: %w", file.Path, err)
		}
	}
	for _, payload := range release.ManualPayloads {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(payload.Path)))
		if err != nil {
			return fmt.Errorf("read manual payload %s: %w", payload.Path, err)
		}
		key := path.Join(baseKey, payload.Path)
		if err := p.store.Put(ctx, key, data, PutOptions{Immutable: true}); err != nil {
			return fmt.Errorf("publish manual payload %s: %w", payload.Path, err)
		}
	}
	return nil
}

// Activate replaces the mutable release pointer. Callers must invoke it only
// after Upload has successfully stored every immutable object.
func (p *Publisher) Activate(ctx context.Context, release manifest.Manifest) error {
	baseKey := releaseBaseKey(release)
	pointer := manifest.Latest{
		ReleaseIdentity: release.ReleaseIdentity,
		BaseURL:         "/" + baseKey,
	}
	pointerData, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return fmt.Errorf("encode latest pointer: %w", err)
	}
	pointerData = append(pointerData, '\n')
	if err := p.store.Put(ctx, "latest.json", pointerData, PutOptions{}); err != nil {
		return fmt.Errorf("update latest pointer: %w", err)
	}
	return nil
}

func releaseBaseKey(release manifest.Manifest) string {
	return path.Join("releases", release.GameVersion, strconv.FormatInt(release.DataVersion, 10))
}

func (p *Publisher) publishManifest(ctx context.Context, release manifest.Manifest) error {
	baseKey := releaseBaseKey(release)
	manifestData, err := json.MarshalIndent(release, "", "  ")
	if err != nil {
		return fmt.Errorf("encode release manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	manifestKey := path.Join(baseKey, "manifest.json")
	if err := p.store.Put(ctx, manifestKey, manifestData, PutOptions{Immutable: true}); err != nil {
		return fmt.Errorf("publish release manifest: %w", err)
	}
	return nil
}
