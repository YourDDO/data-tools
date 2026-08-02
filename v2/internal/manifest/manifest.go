package manifest

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"yourddo-data-tools/v2/internal/contracts"
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/hashing"
)

type File = contracts.GeneratedFileMetadata
type Manifest = contracts.ReleaseManifest
type Latest = contracts.Latest

// Candidate is deterministic, unpublished metadata. DataVersion is
// intentionally absent and is added only when Publisher publishes it.
type Candidate struct {
	SchemaVersion       int                               `json:"schemaVersion"`
	GameVersion         string                            `json:"gameVersion"`
	SourceSHA256        string                            `json:"sourceSha256"`
	MasterDatasetSHA256 string                            `json:"masterDatasetSha256"`
	Domains             []contracts.DatasetMetadata       `json:"domains"`
	GeneratedFiles      []contracts.GeneratedFileMetadata `json:"generatedFiles"`
}

func BuildCandidate(gameVersion, sourceHash, masterHash, root string) (Candidate, error) {
	if strings.TrimSpace(gameVersion) == "" || sourceHash == "" || masterHash == "" {
		return Candidate{}, fmt.Errorf("game version, source hash, and master dataset hash are required")
	}
	paths, err := hashing.Files(root)
	if err != nil {
		return Candidate{}, err
	}
	files := make([]contracts.GeneratedFileMetadata, 0, len(paths))
	for _, path := range paths {
		hash, size, err := hashing.File(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return Candidate{}, err
		}
		domain, err := domainForPath(path)
		if err != nil {
			return Candidate{}, err
		}
		files = append(files, contracts.GeneratedFileMetadata{
			Domain: domain, Path: path, SHA256: hash, SizeBytes: size,
		})
	}
	return Candidate{
		SchemaVersion:       contracts.ReleaseManifestSchemaVersion,
		GameVersion:         gameVersion,
		SourceSHA256:        sourceHash,
		MasterDatasetSHA256: masterHash,
		Domains:             summarize(files),
		GeneratedFiles:      files,
	}, nil
}

func Release(candidate Candidate, dataVersion int64) (Manifest, error) {
	if dataVersion <= 0 {
		return Manifest{}, fmt.Errorf("data version must be a positive Unix timestamp")
	}
	if strings.TrimSpace(candidate.GameVersion) == "" || candidate.MasterDatasetSHA256 == "" {
		return Manifest{}, fmt.Errorf("candidate game version and master dataset hash are required")
	}
	return Manifest{
		SchemaVersion:       candidate.SchemaVersion,
		ReleaseIdentity:     contracts.ReleaseIdentity{GameVersion: candidate.GameVersion, DataVersion: dataVersion},
		MasterDatasetSHA256: candidate.MasterDatasetSHA256,
		Domains:             append([]contracts.DatasetMetadata(nil), candidate.Domains...),
		GeneratedFiles:      append([]contracts.GeneratedFileMetadata(nil), candidate.GeneratedFiles...),
	}, nil
}

func summarize(files []contracts.GeneratedFileMetadata) []contracts.DatasetMetadata {
	type aggregate struct {
		count int
		size  int64
		parts []string
	}
	byDomain := make(map[string]*aggregate)
	for _, file := range files {
		value := byDomain[file.Domain]
		if value == nil {
			value = &aggregate{}
			byDomain[file.Domain] = value
		}
		value.count++
		value.size += file.SizeBytes
		value.parts = append(value.parts, fmt.Sprintf("%s:%d:%s", file.Path, file.SizeBytes, file.SHA256))
	}
	domains := make([]string, 0, len(byDomain))
	for domain := range byDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	result := make([]contracts.DatasetMetadata, 0, len(domains))
	for _, domain := range domains {
		value := byDomain[domain]
		result = append(result, contracts.DatasetMetadata{
			Domain: domain, FileCount: value.count, SizeBytes: value.size, SHA256: hashing.Combine(value.parts...),
		})
	}
	return result
}

func domainForPath(path string) (string, error) {
	first, _, _ := strings.Cut(filepath.ToSlash(path), "/")
	if first == "" || first == "." || first == "candidate.json" || strings.Contains(first, "..") {
		return "", fmt.Errorf("generated file %q is outside a supported release domain", path)
	}
	return first, nil
}

func WriteCandidate(path string, value Candidate) error { return dataset.WriteJSON(path, value, true) }
