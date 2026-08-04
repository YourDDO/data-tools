package manifest

import (
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

// releaseFingerprintSchemaVersion changes whenever domain-generation behavior
// changes without changing the canonical master or manual inputs. Bumping it
// forces existing releases to be regenerated with the current output contract.
const releaseFingerprintSchemaVersion = 3

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
	releaseFingerprint, err := ReleaseFingerprint(masterHash, payloads)
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

// ReleaseFingerprint hashes the output-contract version, canonical master
// hash, and sorted manual payload identity tuples. Size is deliberately
// diagnostic rather than part of release identity.
func ReleaseFingerprint(masterHash string, payloads []contracts.ManualPayloadMetadata) (string, error) {
	if strings.TrimSpace(masterHash) == "" {
		return "", fmt.Errorf("master dataset hash is required")
	}
	ordered := append([]contracts.ManualPayloadMetadata(nil), payloads...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Name != ordered[j].Name {
			return ordered[i].Name < ordered[j].Name
		}
		if ordered[i].Path != ordered[j].Path {
			return ordered[i].Path < ordered[j].Path
		}
		return ordered[i].SHA256 < ordered[j].SHA256
	})
	parts := []string{fmt.Sprintf("release-fingerprint-schema:%d", releaseFingerprintSchemaVersion), masterHash}
	for _, payload := range ordered {
		if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.Path) == "" || strings.TrimSpace(payload.SHA256) == "" {
			return "", fmt.Errorf("manual payload name, path, and hash are required")
		}
		parts = append(parts, hashing.Combine(payload.Name, payload.Path, payload.SHA256))
	}
	return hashing.Combine(parts...), nil
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
