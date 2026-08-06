package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
	"yourddo-data-tools/internal/hashing"
)

type File = contracts.GeneratedFileMetadata
type Manifest = contracts.ReleaseManifest
type Latest = contracts.Latest

// releaseFingerprintSchemaVersion identifies the artifact-manifest format.
// It changes only when the fingerprint algorithm or manifest format changes.
const releaseFingerprintSchemaVersion = 7

type artifactManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Files         []artifactManifestFile `json:"files"`
}

type artifactManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Candidate is deterministic, unpublished metadata. DataVersion is
// intentionally absent and is added only when Publisher publishes it.
type Candidate struct {
	SchemaVersion       int                               `json:"schemaVersion"`
	GameVersion         string                            `json:"gameVersion"`
	SourceSHA256        string                            `json:"sourceSha256"`
	MasterDatasetSHA256 string                            `json:"masterDatasetSha256"`
	ReleaseFingerprint  string                            `json:"releaseFingerprint"`
	ManualPayloads      []contracts.ManualPayloadMetadata `json:"manualPayloads"`
	Domains             []contracts.DatasetMetadata       `json:"domains"`
	GeneratedFiles      []contracts.GeneratedFileMetadata `json:"generatedFiles"`
}

func BuildCandidate(gameVersion, sourceHash, masterHash, root string, manualPayloads []contracts.ManualPayloadMetadata) (Candidate, error) {
	if strings.TrimSpace(gameVersion) == "" || sourceHash == "" || masterHash == "" {
		return Candidate{}, fmt.Errorf("game version, source hash, and master dataset hash are required")
	}
	paths, err := hashing.Files(root)
	if err != nil {
		return Candidate{}, err
	}
	files := make([]contracts.GeneratedFileMetadata, 0, len(paths))
	for _, filePath := range paths {
		if strings.HasPrefix(filePath, "manual/") {
			continue
		}
		hash, size, err := hashing.File(filepath.Join(root, filepath.FromSlash(filePath)))
		if err != nil {
			return Candidate{}, err
		}
		domain, err := domainForPath(filePath)
		if err != nil {
			return Candidate{}, err
		}
		files = append(files, contracts.GeneratedFileMetadata{
			Domain: domain, Path: filePath, SHA256: hash, SizeBytes: size,
		})
	}
	payloads := append([]contracts.ManualPayloadMetadata{}, manualPayloads...)
	sort.Slice(payloads, func(i, j int) bool {
		if payloads[i].Name != payloads[j].Name {
			return payloads[i].Name < payloads[j].Name
		}
		if payloads[i].Path != payloads[j].Path {
			return payloads[i].Path < payloads[j].Path
		}
		return payloads[i].SHA256 < payloads[j].SHA256
	})
	releaseFingerprint, err := ArtifactFingerprint(root, files, payloads)
	if err != nil {
		return Candidate{}, err
	}
	return Candidate{
		SchemaVersion:       contracts.ReleaseManifestSchemaVersion,
		GameVersion:         gameVersion,
		SourceSHA256:        sourceHash,
		MasterDatasetSHA256: masterHash,
		ReleaseFingerprint:  releaseFingerprint,
		ManualPayloads:      payloads,
		Domains:             summarize(files),
		GeneratedFiles:      files,
	}, nil
}

func Release(candidate Candidate, dataVersion int64) (Manifest, error) {
	if dataVersion <= 0 {
		return Manifest{}, fmt.Errorf("data version must be a positive Unix timestamp")
	}
	if strings.TrimSpace(candidate.GameVersion) == "" || candidate.MasterDatasetSHA256 == "" || candidate.ReleaseFingerprint == "" {
		return Manifest{}, fmt.Errorf("candidate game version, master dataset hash, and release fingerprint are required")
	}
	return Manifest{
		SchemaVersion:       candidate.SchemaVersion,
		ReleaseIdentity:     contracts.ReleaseIdentity{GameVersion: candidate.GameVersion, DataVersion: dataVersion},
		MasterDatasetSHA256: candidate.MasterDatasetSHA256,
		ReleaseFingerprint:  candidate.ReleaseFingerprint,
		ManualPayloads:      append([]contracts.ManualPayloadMetadata(nil), candidate.ManualPayloads...),
		Domains:             append([]contracts.DatasetMetadata(nil), candidate.Domains...),
		GeneratedFiles:      append([]contracts.GeneratedFileMetadata(nil), candidate.GeneratedFiles...),
	}, nil
}

// PublishablePaths returns the release content objects uploaded by Publisher.
// The release manifest itself is metadata containing the fingerprint and the
// publication timestamp, so it is deliberately not part of this set.
func PublishablePaths(files []contracts.GeneratedFileMetadata, payloads []contracts.ManualPayloadMetadata) ([]string, error) {
	paths := make([]string, 0, len(files)+len(payloads))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	for _, payload := range payloads {
		paths = append(paths, payload.Path)
	}
	seen := make(map[string]struct{}, len(paths))
	for index, value := range paths {
		cleaned := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(value, `\`, "/")))
		if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("publishable artifact path %q escapes the release root", value)
		}
		normalized := filepath.ToSlash(cleaned)
		if _, exists := seen[normalized]; exists {
			return nil, fmt.Errorf("publishable artifact path %q is duplicated", normalized)
		}
		seen[normalized] = struct{}{}
		paths[index] = normalized
	}
	sort.Strings(paths)
	return paths, nil
}

// ArtifactFingerprint hashes a deterministic manifest of release-relative
// publishable paths and the SHA-256 digest of each file's bytes.
func ArtifactFingerprint(root string, files []contracts.GeneratedFileMetadata, payloads []contracts.ManualPayloadMetadata) (string, error) {
	paths, err := PublishablePaths(files, payloads)
	if err != nil {
		return "", err
	}
	value := artifactManifest{SchemaVersion: releaseFingerprintSchemaVersion, Files: make([]artifactManifestFile, 0, len(paths))}
	for _, relative := range paths {
		digest, _, err := hashing.File(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return "", fmt.Errorf("fingerprint publishable artifact %s: %w", relative, err)
		}
		value.Files = append(value.Files, artifactManifestFile{Path: relative, SHA256: digest})
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode artifact manifest: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
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

func WriteCandidate(path string, value Candidate) error { return dataset.WriteJSON(path, value) }
