// Package manual discovers and canonicalizes manually maintained JSON payloads.
package manual

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
)

// Prepare discovers JSON payloads beneath sourceRoot, validates and
// canonicalizes them as compact JSON, and writes the exact bytes to destinationRoot.
func Prepare(sourceRoot, destinationRoot string) ([]contracts.ManualPayloadMetadata, error) {
	if strings.TrimSpace(sourceRoot) == "" {
		return nil, fmt.Errorf("manual input directory is required")
	}
	if strings.TrimSpace(destinationRoot) == "" {
		return nil, fmt.Errorf("manual payload destination is required")
	}
	info, err := os.Stat(sourceRoot)
	if os.IsNotExist(err) {
		return []contracts.ManualPayloadMetadata{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect manual input directory %s: %w", sourceRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("manual input path %s is not a directory", sourceRoot)
	}

	paths := make([]string, 0)
	err = filepath.WalkDir(sourceRoot, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular manual input %s is not supported", filePath)
		}
		relative, err := filepath.Rel(sourceRoot, filePath)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if path.Ext(relative) == ".json" {
			paths = append(paths, relative)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover manual payloads beneath %s: %w", sourceRoot, err)
	}
	sort.Strings(paths)

	result := make([]contracts.ManualPayloadMetadata, 0, len(paths))
	for _, relative := range paths {
		name := strings.TrimSuffix(relative, ".json")
		if name == "" || name == "." {
			return nil, fmt.Errorf("manual payload %q has no logical name", relative)
		}
		sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(relative))
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read manual payload %s: %w", relative, err)
		}
		canonical, err := Canonicalize(data)
		if err != nil {
			return nil, fmt.Errorf("decode manual payload %s: %w", relative, err)
		}
		destination := filepath.Join(destinationRoot, filepath.FromSlash(relative))
		if err := dataset.WriteData(destination, canonical); err != nil {
			return nil, fmt.Errorf("write canonical manual payload %s: %w", relative, err)
		}
		digest := sha256.Sum256(canonical)
		result = append(result, contracts.ManualPayloadMetadata{
			Name: name, Path: path.Join("manual", relative),
			SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(canonical)),
		})
	}
	return result, nil
}

// Canonicalize decodes exactly one JSON value, rejects duplicate object keys,
// emits compact JSON with object keys sorted through encoding/json, preserves
// array order, and appends the repository-standard trailing newline.
func Canonicalize(data []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, fmt.Errorf("trailing data: %w", err)
	}

	duplicateDecoder := json.NewDecoder(bytes.NewReader(data))
	duplicateDecoder.UseNumber()
	if err := scanValue(duplicateDecoder); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(canonical, '\n'), nil
}

func scanValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	_, err = decoder.Token()
	return err
}
