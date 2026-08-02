package validation

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yourddo-data-tools/v2/internal/contracts"
	"yourddo-data-tools/v2/internal/dataset"
	"yourddo-data-tools/v2/internal/domain/registry"
	"yourddo-data-tools/v2/internal/hashing"
	"yourddo-data-tools/v2/internal/manifest"
	"yourddo-data-tools/v2/internal/manual"
)

const (
	defaultMaximumReduction = 0.50
	defaultMaximumIncrease  = 1.00
	minimumDriftBaseline    = 10
)

// Options controls policy without changing the generated candidate. Baseline
// comparisons are relative, with a small-record floor to avoid brittle checks
// that become obsolete whenever a fixture gains one record.
type Options struct {
	WarningsAsErrors bool
	BaselineRoot     string
	Baseline         *manifest.Candidate
	MaximumReduction float64
	MaximumIncrease  float64
}

func normalizeOptions(options Options) Options {
	if options.MaximumReduction <= 0 {
		options.MaximumReduction = defaultMaximumReduction
	}
	if options.MaximumIncrease <= 0 {
		options.MaximumIncrease = defaultMaximumIncrease
	}
	return options
}

func JSONDirectory(root string) error {
	report := jsonDirectoryReport(root)
	report.sort()
	return report.Err(false)
}

func Master(root string) error {
	report := MasterReport(root)
	return report.Err(false)
}

func MasterReport(root string) Report {
	var report Report
	indexPath := filepath.Join(root, dataset.MasterIndexName)
	index, err := dataset.LoadMasterIndex(root)
	if err != nil {
		report.add(Error, "master", dataset.MasterIndexName, "<file>", "valid-json", err.Error())
		report.merge(jsonDirectoryReport(root))
		report.sort()
		return report
	}
	if index.SchemaVersion != 1 {
		report.add(Error, "master", dataset.MasterIndexName, "<file>", "valid-enum", "schemaVersion must be 1")
	}
	seen := make(map[string]struct{}, len(index.Files))
	indexed := make(map[string]struct{}, len(index.Files))
	validKinds := map[string]struct{}{"items": {}, "augments": {}, "filigree-sets": {}}
	for position, entry := range index.Files {
		recordID := fmt.Sprintf("files[%d]", position)
		if entry.Category == "" || entry.Kind == "" || entry.Path == "" {
			report.add(Error, "master", dataset.MasterIndexName, recordID, "required-fields", "category, kind, and path are required")
			continue
		}
		if _, ok := validKinds[entry.Kind]; !ok {
			report.add(Error, "master", dataset.MasterIndexName, recordID, "valid-enum", fmt.Sprintf("kind %q is not supported", entry.Kind))
		}
		if _, exists := seen[entry.Path]; exists {
			report.add(Error, "master", dataset.MasterIndexName, entry.Path, "unique-identifier", "master index path is duplicated")
		}
		seen[entry.Path] = struct{}{}
		cleaned, ok := cleanRelative(entry.Path)
		if !ok {
			report.add(Error, "master", dataset.MasterIndexName, entry.Path, "safe-path", "path escapes the dataset root")
			continue
		}
		indexed[filepath.ToSlash(cleaned)] = struct{}{}
		if _, err := os.Stat(filepath.Join(root, cleaned)); err != nil {
			report.add(Error, "master", entry.Path, "<file>", "manifest-file-agreement", err.Error())
		}
	}
	paths, err := hashing.Files(root)
	if err != nil {
		report.add(Error, "master", "<directory>", "<file>", "readable-files", err.Error())
	} else {
		for _, path := range paths {
			if path == dataset.MasterIndexName {
				continue
			}
			if _, exists := indexed[path]; !exists {
				report.add(Error, "master", path, "<file>", "manifest-file-agreement", "file is not declared by master-index.json")
			}
		}
	}
	_ = indexPath
	report.merge(jsonDirectoryReport(root))
	report.merge(validateMasterRecords(root, index))
	report.sort()
	return report
}

func GeneratedFiles(root string, files []contracts.GeneratedFileMetadata) error {
	report := generatedFilesReport(root, files, nil, false)
	report.merge(validateRecordFiles(root, files, nil))
	report.sort()
	return report.Err(false)
}

// Domains validates a previously generated selection when write-time metadata
// is unavailable (for example, the standalone validation command).
func Domains(root string, selected []string) error {
	var report Report
	files := make([]contracts.GeneratedFileMetadata, 0)
	generators, err := registry.ResolveAll(selected)
	if err != nil {
		report.add(Error, "domains", "<selection>", "<file>", "valid-enum", err.Error())
	}
	for _, generator := range generators {
		domainRoot := filepath.Join(root, generator.Name())
		if info, err := os.Stat(domainRoot); err != nil {
			report.add(Error, generator.Name(), generator.Name(), "<file>", "required-dataset", err.Error())
		} else if !info.IsDir() {
			report.add(Error, generator.Name(), generator.Name(), "<file>", "required-dataset", "domain output is not a directory")
		} else if paths, err := hashing.Files(domainRoot); err != nil {
			report.add(Error, generator.Name(), generator.Name(), "<file>", "readable-files", err.Error())
		} else {
			for _, relative := range paths {
				path := filepath.Join(domainRoot, filepath.FromSlash(relative))
				hash, size, err := hashing.File(path)
				if err != nil {
					report.add(Error, generator.Name(), filepath.ToSlash(filepath.Join(generator.Name(), relative)), "<file>", "readable-files", err.Error())
					continue
				}
				files = append(files, contracts.GeneratedFileMetadata{
					Domain: generator.Name(), Path: filepath.ToSlash(filepath.Join(generator.Name(), relative)), SHA256: hash, SizeBytes: size,
				})
			}
		}
	}
	report.merge(jsonDirectoryReport(root))
	report.merge(validateRecordFiles(root, files, nil))
	report.sort()
	return report.Err(false)
}

func Release(root string, value manifest.Manifest) error {
	report := ReleaseReport(root, value, Options{})
	return report.Err(false)
}

func ReleaseReport(root string, value manifest.Manifest, options Options) Report {
	var report Report
	options = normalizeOptions(options)
	if value.SchemaVersion != contracts.ReleaseManifestSchemaVersion || value.GameVersion == "" || value.DataVersion <= 0 || value.MasterDatasetSHA256 == "" || value.ReleaseFingerprint == "" {
		report.add(Error, "release", "manifest.json", "<file>", "required-fields", "release identity, schema version, master hash, and release fingerprint must be complete")
	}
	report.merge(generatedFilesReport(root, value.GeneratedFiles, value.ManualPayloads, true))
	report.merge(manualPayloadsReport(root, value.ManualPayloads, "manifest.json"))
	report.merge(validateDomainSummaries(value.Domains, value.GeneratedFiles, "manifest.json"))
	report.merge(validateCandidateContents(root, value.MasterDatasetSHA256, value.ReleaseFingerprint, value.GeneratedFiles, value.ManualPayloads, options))
	report.sort()
	return report
}

func Candidate(root string, value manifest.Candidate) error {
	report := CandidateReport(root, value, Options{})
	return report.Err(false)
}

func CandidateReport(root string, value manifest.Candidate, options Options) Report {
	var report Report
	options = normalizeOptions(options)
	if value.SchemaVersion != contracts.ReleaseManifestSchemaVersion || value.GameVersion == "" || value.SourceSHA256 == "" || value.MasterDatasetSHA256 == "" || value.ReleaseFingerprint == "" {
		report.add(Error, "release", "candidate.json", "<file>", "required-fields", "candidate identity, schema version, source hash, master hash, and release fingerprint must be complete")
	}
	report.merge(generatedFilesReport(root, value.GeneratedFiles, value.ManualPayloads, true))
	report.merge(manualPayloadsReport(root, value.ManualPayloads, "candidate.json"))
	report.merge(validateDomainSummaries(value.Domains, value.GeneratedFiles, "candidate.json"))
	report.merge(validateCandidateContents(root, value.MasterDatasetSHA256, value.ReleaseFingerprint, value.GeneratedFiles, value.ManualPayloads, options))
	report.sort()
	return report
}

func validateCandidateContents(root, expectedMasterHash, expectedReleaseFingerprint string, files []manifest.File, payloads []contracts.ManualPayloadMetadata, options Options) Report {
	var report Report
	masterRoot := filepath.Join(root, "master")
	if hash, err := hashing.Directory(masterRoot); err != nil {
		report.add(Error, "master", "master", "<file>", "master-hash-agreement", err.Error())
	} else if hash != expectedMasterHash {
		report.add(Error, "master", "master", "<file>", "master-hash-agreement", "master dataset hash does not match candidate metadata")
	}
	if fingerprint, err := manifest.ReleaseFingerprint(expectedMasterHash, payloads); err != nil {
		report.add(Error, "release", "manifest.json", "<file>", "release-fingerprint-agreement", err.Error())
	} else if fingerprint != expectedReleaseFingerprint {
		report.add(Error, "release", "manifest.json", "<file>", "release-fingerprint-agreement", "release fingerprint does not match master and manual payload metadata")
	}
	report.merge(MasterReport(masterRoot))
	master, err := dataset.LoadMaster(masterRoot)
	if err != nil {
		report.add(Error, "master", "master/master-index.json", "<file>", "master-contract", err.Error())
	} else {
		report.merge(validateRecordFiles(root, files, &master))
	}
	if options.Baseline != nil && options.BaselineRoot != "" {
		report.merge(validateCountDrift(root, files, options))
	}
	return report
}

func generatedFilesReport(root string, files []manifest.File, payloads []contracts.ManualPayloadMetadata, exact bool) Report {
	var report Report
	seen := make(map[string]struct{}, len(files))
	expected := make(map[string]struct{}, len(files))
	for position, entry := range files {
		datasetName := entry.Domain
		if datasetName == "" {
			datasetName = "release"
		}
		recordID := fmt.Sprintf("generatedFiles[%d]", position)
		if entry.Domain == "" || entry.Path == "" || entry.SHA256 == "" || entry.SizeBytes <= 0 {
			report.add(Error, datasetName, "candidate.json", recordID, "required-fields", "domain, path, positive sizeBytes, and sha256 are required")
		}
		if _, exists := seen[entry.Path]; exists {
			report.add(Error, datasetName, entry.Path, "<file>", "unique-identifier", "generated file path is duplicated")
		}
		seen[entry.Path] = struct{}{}
		cleaned, ok := cleanRelative(entry.Path)
		if !ok {
			report.add(Error, datasetName, entry.Path, "<file>", "safe-path", "path escapes the release root")
			continue
		}
		path := filepath.ToSlash(cleaned)
		expected[path] = struct{}{}
		first, _, _ := strings.Cut(path, "/")
		if first != entry.Domain {
			report.add(Error, datasetName, entry.Path, "<file>", "manifest-file-agreement", "domain does not agree with the file path")
		}
		if !validSHA256(entry.SHA256) {
			report.add(Error, datasetName, entry.Path, "<file>", "file-hash-agreement", "sha256 must be a 64-character hexadecimal digest")
		}
		hash, size, err := hashing.File(filepath.Join(root, cleaned))
		if err != nil {
			report.add(Error, datasetName, entry.Path, "<file>", "manifest-file-agreement", err.Error())
			continue
		}
		if hash != entry.SHA256 {
			report.add(Error, datasetName, entry.Path, "<file>", "file-hash-agreement", "file SHA-256 does not match the manifest")
		}
		if size != entry.SizeBytes {
			report.add(Error, datasetName, entry.Path, "<file>", "file-size-agreement", "file size does not match the manifest")
		}
	}
	for _, payload := range payloads {
		cleaned, ok := cleanRelative(payload.Path)
		if ok {
			expected[filepath.ToSlash(cleaned)] = struct{}{}
		}
	}
	paths, err := hashing.Files(root)
	if err != nil {
		report.add(Error, "release", "<directory>", "<file>", "readable-files", err.Error())
		return report
	}
	for _, path := range paths {
		if temporaryOrPartial(path) {
			report.add(Error, datasetForPath(path), path, "<file>", "release-file-hygiene", "temporary or partial files must not be included in a release")
		}
		if exact && path != "candidate.json" && path != "manifest.json" {
			if _, exists := expected[path]; !exists {
				report.add(Error, datasetForPath(path), path, "<file>", "manifest-file-agreement", "file is present but not declared in the manifest")
			}
		}
	}
	report.merge(jsonDirectoryReport(root))
	return report
}

func manualPayloadsReport(root string, payloads []contracts.ManualPayloadMetadata, metadataFile string) Report {
	var report Report
	seenNames := make(map[string]struct{}, len(payloads))
	seenPaths := make(map[string]struct{}, len(payloads))
	for position, payload := range payloads {
		recordID := fmt.Sprintf("manualPayloads[%d]", position)
		if payload.Name == "" || payload.Path == "" || payload.SHA256 == "" || payload.SizeBytes <= 0 {
			report.add(Error, "manual", metadataFile, recordID, "required-fields", "name, path, positive sizeBytes, and sha256 are required")
		}
		if _, exists := seenNames[payload.Name]; exists {
			report.add(Error, "manual", metadataFile, payload.Name, "unique-identifier", "manual payload name is duplicated")
		}
		seenNames[payload.Name] = struct{}{}
		if _, exists := seenPaths[payload.Path]; exists {
			report.add(Error, "manual", metadataFile, payload.Path, "unique-identifier", "manual payload path is duplicated")
		}
		seenPaths[payload.Path] = struct{}{}
		cleaned, ok := cleanRelative(payload.Path)
		if !ok || !strings.HasPrefix(filepath.ToSlash(cleaned), "manual/") || filepath.Ext(cleaned) != ".json" {
			report.add(Error, "manual", payload.Path, "<file>", "safe-path", "manual payload path must be a JSON file beneath manual/")
			continue
		}
		relativeName := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(cleaned), "manual/"), ".json")
		if payload.Name != relativeName {
			report.add(Error, "manual", payload.Path, "<file>", "manifest-file-agreement", "logical name does not agree with the release path")
		}
		if !validSHA256(payload.SHA256) {
			report.add(Error, "manual", payload.Path, "<file>", "file-hash-agreement", "sha256 must be a 64-character hexadecimal digest")
		}
		data, err := os.ReadFile(filepath.Join(root, cleaned))
		if err != nil {
			report.add(Error, "manual", payload.Path, "<file>", "manifest-file-agreement", err.Error())
			continue
		}
		canonical, err := manual.Canonicalize(data)
		if err != nil {
			report.add(Error, "manual", payload.Path, "<file>", "valid-json", err.Error())
		} else if !bytes.Equal(canonical, data) {
			report.add(Error, "manual", payload.Path, "<file>", "canonical-json", "manual payload bytes are not canonical")
		}
		hash, size, err := hashing.File(filepath.Join(root, cleaned))
		if err != nil {
			continue
		}
		if hash != payload.SHA256 {
			report.add(Error, "manual", payload.Path, "<file>", "file-hash-agreement", "file SHA-256 does not match the manifest")
		}
		if size != payload.SizeBytes {
			report.add(Error, "manual", payload.Path, "<file>", "file-size-agreement", "file size does not match the manifest")
		}
	}
	return report
}

func validateDomainSummaries(domains []contracts.DatasetMetadata, files []manifest.File, metadataFile string) Report {
	type aggregate struct {
		count int
		size  int64
		parts []string
	}
	var report Report
	actual := make(map[string]*aggregate)
	for _, file := range files {
		value := actual[file.Domain]
		if value == nil {
			value = &aggregate{}
			actual[file.Domain] = value
		}
		value.count++
		value.size += file.SizeBytes
		value.parts = append(value.parts, fmt.Sprintf("%s:%d:%s", file.Path, file.SizeBytes, file.SHA256))
	}
	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		if _, exists := seen[domain.Domain]; exists {
			report.add(Error, domain.Domain, metadataFile, domain.Domain, "unique-identifier", "domain summary is duplicated")
		}
		seen[domain.Domain] = struct{}{}
		value := actual[domain.Domain]
		if domain.Domain == "" || value == nil {
			report.add(Error, "release", metadataFile, domain.Domain, "manifest-file-agreement", "domain summary has no matching generated files")
			continue
		}
		if domain.FileCount != value.count || domain.SizeBytes != value.size || domain.SHA256 != hashing.Combine(value.parts...) {
			report.add(Error, domain.Domain, metadataFile, domain.Domain, "manifest-file-agreement", "domain summary does not match its generated files")
		}
	}
	for domain := range actual {
		if _, exists := seen[domain]; !exists {
			report.add(Error, domain, metadataFile, domain, "manifest-file-agreement", "generated files have no domain summary")
		}
	}
	return report
}

func jsonDirectoryReport(root string) Report {
	var report Report
	paths, err := hashing.Files(root)
	if err != nil {
		report.add(Error, datasetForPath(root), "<directory>", "<file>", "readable-files", err.Error())
		return report
	}
	for _, relative := range paths {
		if filepath.Ext(relative) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			report.add(Error, datasetForPath(relative), relative, "<file>", "valid-json", err.Error())
			continue
		}
		if !json.Valid(data) {
			report.add(Error, datasetForPath(relative), relative, "<file>", "valid-json", "file contains invalid JSON or is truncated")
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			report.add(Error, datasetForPath(relative), relative, "<file>", "canonical-newline", "JSON file must end with a newline")
		}
		if json.Valid(data) {
			for _, key := range duplicateJSONKeys(data) {
				report.add(Error, datasetForPath(relative), relative, key, "unique-identifier", "JSON object contains a duplicate key")
			}
		}
	}
	return report
}

func duplicateJSONKeys(data []byte) []string {
	decoder := json.NewDecoder(bytes.NewReader(data))
	duplicates := make([]string, 0)
	scanJSONValue(decoder, &duplicates)
	sort.Strings(duplicates)
	return duplicates
}

func scanJSONValue(decoder *json.Decoder, duplicates *[]string) {
	token, err := decoder.Token()
	if err != nil {
		return
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return
			}
			key, _ := keyToken.(string)
			if _, exists := seen[key]; exists {
				*duplicates = append(*duplicates, key)
			}
			seen[key] = struct{}{}
			scanJSONValue(decoder, duplicates)
		}
		_, _ = decoder.Token()
	case '[':
		for decoder.More() {
			scanJSONValue(decoder, duplicates)
		}
		_, _ = decoder.Token()
	}
}

func cleanRelative(path string) (string, bool) {
	cleaned := filepath.Clean(filepath.FromSlash(path))
	return cleaned, cleaned != "." && !filepath.IsAbs(cleaned) && cleaned != ".." && !strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && strings.ToLower(value) == value
}

func datasetForPath(path string) string {
	path = filepath.ToSlash(path)
	first, _, _ := strings.Cut(path, "/")
	if first == "" || first == "." || first == "<directory>" {
		return "release"
	}
	return first
}

func temporaryOrPartial(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") ||
		strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".temp") ||
		strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".partial")
}

func DecodeCandidate(path string) (manifest.Candidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.Candidate{}, fmt.Errorf("read candidate metadata %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value manifest.Candidate
	if err := decoder.Decode(&value); err != nil {
		return manifest.Candidate{}, fmt.Errorf("decode candidate metadata %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return manifest.Candidate{}, fmt.Errorf("decode candidate metadata %s: trailing JSON value", path)
	}
	return value, nil
}

func DecodeManifest(path string) (manifest.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("read manifest %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value manifest.Manifest
	if err := decoder.Decode(&value); err != nil {
		return manifest.Manifest{}, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return manifest.Manifest{}, fmt.Errorf("decode manifest %s: trailing JSON value", path)
	}
	return value, nil
}
