package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"yourddo-data-tools/v2/internal/manifest"
)

// ActiveMasterHash returns the hash named by the active release, or available
// false when this publication root has not been activated yet.
func (s *LocalStore) ActiveMasterHash(ctx context.Context) (hash string, available bool, returnErr error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	pointerData, err := os.ReadFile(filepath.Join(s.root, "latest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read active release pointer: %w", err)
	}
	var pointer manifest.Latest
	if err := decodeStrict(pointerData, &pointer); err != nil {
		return "", false, fmt.Errorf("decode active release pointer: %w", err)
	}
	relative := strings.TrimPrefix(pointer.BaseURL, "/")
	cleaned := filepath.Clean(filepath.FromSlash(relative))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("active release pointer contains an unsafe base path")
	}
	manifestData, err := os.ReadFile(filepath.Join(s.root, cleaned, "manifest.json"))
	if err != nil {
		return "", false, fmt.Errorf("read active release manifest: %w", err)
	}
	var release manifest.Manifest
	if err := decodeStrict(manifestData, &release); err != nil {
		return "", false, fmt.Errorf("decode active release manifest: %w", err)
	}
	if release.ReleaseIdentity != pointer.ReleaseIdentity {
		return "", false, fmt.Errorf("active release pointer does not match its manifest")
	}
	if release.MasterDatasetSHA256 == "" {
		return "", false, fmt.Errorf("active release manifest has no master dataset hash")
	}
	return release.MasterDatasetSHA256, true, nil
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
