package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Assemble creates a complete, create-only local release. The caller assigns
// dataVersion only after generation and validation have established that the
// master dataset changed.
func Assemble(candidateRoot, releaseRoot string, candidate Candidate, dataVersion int64) (result Manifest, returnErr error) {
	result, err := Release(candidate, dataVersion)
	if err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(releaseRoot) == "" || filepath.Clean(releaseRoot) == "." {
		return Manifest{}, fmt.Errorf("local release directory is required")
	}
	if err := os.Mkdir(releaseRoot, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return Manifest{}, fmt.Errorf("local release directory %s already exists; refusing to overwrite it", releaseRoot)
		}
		return Manifest{}, fmt.Errorf("create local release directory: %w", err)
	}
	complete := false
	defer func() {
		if complete {
			return
		}
		if cleanupErr := os.RemoveAll(releaseRoot); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove incomplete local release: %w", cleanupErr))
		}
	}()
	for _, file := range result.GeneratedFiles {
		relative, err := safeRelative(file.Path)
		if err != nil {
			return Manifest{}, err
		}
		if err := copyCreateOnly(filepath.Join(candidateRoot, relative), filepath.Join(releaseRoot, relative)); err != nil {
			return Manifest{}, fmt.Errorf("assemble release file %s: %w", file.Path, err)
		}
	}
	manifestData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("encode release manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := writeCreateOnly(filepath.Join(releaseRoot, "manifest.json"), manifestData); err != nil {
		return Manifest{}, fmt.Errorf("write release manifest: %w", err)
	}
	complete = true
	return result, nil
}

func safeRelative(value string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(value))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("release file path %q escapes the release root", value)
	}
	return cleaned, nil
}

func copyCreateOnly(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		output.Close()
		if remove {
			os.Remove(destination)
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	remove = false
	return nil
}

func writeCreateOnly(destination string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
