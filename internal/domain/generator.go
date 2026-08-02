// Package domain defines the source-independent contract implemented by every
// domain dataset generator.
package domain

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/hashing"
)

// Result reports every file written by a generator and any non-fatal data
// quality warnings. File paths are relative to the common output root.
type Result struct {
	Domain   string
	Files    []contracts.GeneratedFileMetadata
	Warnings []string
}

// Generator is intentionally independent of Compendium and all network
// clients. A future domain only needs this interface and one registry entry.
type Generator interface {
	Name() string
	Generate(context.Context, dataset.Master, string) (Result, error)
}

// WriteJSON writes one deterministic domain file and returns its hash metadata.
func WriteJSON(outputRoot, domainName, relative string, value any) (contracts.GeneratedFileMetadata, error) {
	path, relativePath, err := outputPath(outputRoot, domainName, relative)
	if err != nil {
		return contracts.GeneratedFileMetadata{}, err
	}
	if err := dataset.WriteJSON(path, value, false); err != nil {
		return contracts.GeneratedFileMetadata{}, err
	}
	return metadata(domainName, relativePath, path)
}

// WriteCanonical writes canonical bytes without reinterpreting their schema.
func WriteCanonical(outputRoot, domainName, relative string, data []byte) (contracts.GeneratedFileMetadata, error) {
	path, relativePath, err := outputPath(outputRoot, domainName, relative)
	if err != nil {
		return contracts.GeneratedFileMetadata{}, err
	}
	if err := dataset.WriteData(path, data); err != nil {
		return contracts.GeneratedFileMetadata{}, err
	}
	return metadata(domainName, relativePath, path)
}

func outputPath(outputRoot, domainName, relative string) (string, string, error) {
	domainName = strings.TrimSpace(domainName)
	cleaned := filepath.Clean(filepath.FromSlash(relative))
	if domainName == "" || domainName == "." || domainName == ".." || strings.ContainsAny(domainName, `/\`) ||
		cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("domain %q has invalid output path %q", domainName, relative)
	}
	relativePath := filepath.ToSlash(filepath.Join(domainName, cleaned))
	return filepath.Join(outputRoot, filepath.FromSlash(relativePath)), relativePath, nil
}

func metadata(domainName, relativePath, path string) (contracts.GeneratedFileMetadata, error) {
	hash, size, err := hashing.File(path)
	if err != nil {
		return contracts.GeneratedFileMetadata{}, err
	}
	return contracts.GeneratedFileMetadata{Domain: domainName, Path: relativePath, SHA256: hash, SizeBytes: size}, nil
}

// SortResult makes generator metadata and warnings stable even when a domain
// writes files by walking a map or another unordered collection.
func SortResult(result *Result) {
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	sort.Strings(result.Warnings)
}
