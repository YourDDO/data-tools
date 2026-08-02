package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MasterIndexName = "master-index.json"

type MasterFile struct {
	Category string `json:"category"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
}

type MasterIndex struct {
	SchemaVersion int          `json:"schemaVersion"`
	Files         []MasterFile `json:"files"`
}

func ReadJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func WriteJSON(path string, value any, indent bool) error {
	var (
		data []byte
		err  error
	)
	if indent {
		data, err = json.MarshalIndent(value, "", "  ")
	} else {
		data, err = json.Marshal(value)
	}
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')
	return WriteData(path, data)
}

// WriteData atomically writes already-serialized deterministic dataset bytes.
func WriteData(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent for %s: %w", path, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".dataset-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("set mode for %s: %w", path, err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func LoadMasterIndex(root string) (MasterIndex, error) {
	var index MasterIndex
	if err := ReadJSON(filepath.Join(root, MasterIndexName), &index); err != nil {
		return MasterIndex{}, err
	}
	return index, nil
}

func WriteMasterIndex(root string, index MasterIndex) error {
	sort.Slice(index.Files, func(i, j int) bool {
		if index.Files[i].Kind != index.Files[j].Kind {
			return index.Files[i].Kind < index.Files[j].Kind
		}
		return index.Files[i].Path < index.Files[j].Path
	})
	return WriteJSON(filepath.Join(root, MasterIndexName), index, true)
}

func LoadItems(root string) ([]ItemData, error) {
	index, err := LoadMasterIndex(root)
	if err != nil {
		return nil, err
	}
	items := make([]ItemData, 0)
	for _, entry := range index.Files {
		if entry.Kind != "items" {
			continue
		}
		var fileItems []ItemData
		path, err := secureJoin(root, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("master entry %q: %w", entry.Path, err)
		}
		if err := ReadJSON(path, &fileItems); err != nil {
			return nil, err
		}
		items = append(items, fileItems...)
	}
	return items, nil
}

func secureJoin(root, relative string) (string, error) {
	cleaned := filepath.Clean(relative)
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes dataset root")
	}
	return filepath.Join(root, cleaned), nil
}
