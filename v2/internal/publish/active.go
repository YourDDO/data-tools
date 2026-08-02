package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"yourddo-data-tools/v2/internal/manifest"
)

type ActiveMaster struct {
	LatestObjectKey   string
	ActiveManifestKey string
	MasterSHA256      string
}

// ActiveMasterHash returns the hash named by the active release, or available
// false when this publication root has not been activated yet.
func (s *LocalStore) ActiveMasterHash(ctx context.Context) (active ActiveMaster, available bool, returnErr error) {
	active.LatestObjectKey = "latest.json"
	if err := ctx.Err(); err != nil {
		return active, false, err
	}
	pointerData, err := os.ReadFile(filepath.Join(s.root, active.LatestObjectKey))
	if errors.Is(err, os.ErrNotExist) {
		return active, false, nil
	}
	if err != nil {
		return active, false, fmt.Errorf("read active release pointer: %w", err)
	}
	var pointer manifest.Latest
	if err := decodeStrict(pointerData, &pointer); err != nil {
		return active, false, fmt.Errorf("decode active release pointer: %w", err)
	}
	manifestKey, err := activeManifestKey(pointer)
	if err != nil {
		return active, false, err
	}
	active.ActiveManifestKey = manifestKey
	manifestPath, err := s.path(manifestKey)
	if err != nil {
		return active, false, fmt.Errorf("resolve active release manifest: %w", err)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return active, false, fmt.Errorf("read active release manifest: %w", err)
	}
	var release manifest.Manifest
	if err := decodeStrict(manifestData, &release); err != nil {
		return active, false, fmt.Errorf("decode active release manifest: %w", err)
	}
	if release.ReleaseIdentity != pointer.ReleaseIdentity {
		return active, false, fmt.Errorf("active release pointer does not match its manifest")
	}
	if release.MasterDatasetSHA256 == "" {
		return active, false, fmt.Errorf("active release manifest has no master dataset hash")
	}
	active.MasterSHA256 = release.MasterDatasetSHA256
	return active, true, nil
}

func (s *S3Store) ActiveMasterHash(ctx context.Context) (active ActiveMaster, available bool, returnErr error) {
	active.LatestObjectKey = "latest.json"
	pointerData, err := s.get(ctx, active.LatestObjectKey)
	if isNoSuchKey(err) {
		return active, false, nil
	}
	if err != nil {
		return active, false, fmt.Errorf("read active release pointer: %w", err)
	}
	var pointer manifest.Latest
	if err := decodeStrict(pointerData, &pointer); err != nil {
		return active, false, fmt.Errorf("decode active release pointer: %w", err)
	}
	manifestKey, err := activeManifestKey(pointer)
	if err != nil {
		return active, false, err
	}
	active.ActiveManifestKey = manifestKey
	manifestData, err := s.get(ctx, manifestKey)
	if err != nil {
		return active, false, fmt.Errorf("read active release manifest: %w", err)
	}
	var release manifest.Manifest
	if err := decodeStrict(manifestData, &release); err != nil {
		return active, false, fmt.Errorf("decode active release manifest: %w", err)
	}
	if release.ReleaseIdentity != pointer.ReleaseIdentity {
		return active, false, fmt.Errorf("active release pointer does not match its manifest")
	}
	if release.MasterDatasetSHA256 == "" {
		return active, false, fmt.Errorf("active release manifest has no master dataset hash")
	}
	active.MasterSHA256 = release.MasterDatasetSHA256
	return active, true, nil
}

func activeManifestKey(pointer manifest.Latest) (string, error) {
	relative := strings.TrimPrefix(pointer.BaseURL, "/")
	cleaned := path.Clean(relative)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, `\`) {
		return "", fmt.Errorf("active release pointer contains an unsafe base path")
	}
	manifestKey := path.Join(cleaned, "manifest.json")
	if err := validateS3Key(manifestKey); err != nil {
		return "", fmt.Errorf("active release pointer contains an unsafe base path: %w", err)
	}
	return manifestKey, nil
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}
