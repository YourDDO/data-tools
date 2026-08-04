package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ItemRecord pairs a canonical item with the indexed file that supplied it.
type ItemRecord struct {
	Category string
	File     string
	Item     ItemData
	Raw      json.RawMessage
}

func (r ItemRecord) Source() string { return recordSource(r.File, r.Item.PageTitle, r.Item.Name) }

// AugmentRecord pairs a canonical augment with its indexed master file.
type AugmentRecord struct {
	Category string
	File     string
	Augment  AugmentItem
}

func (r AugmentRecord) Source() string { return recordSource(r.File, r.Augment.Name, "") }

func recordSource(file, identifier, fallback string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(fallback)
	}
	if identifier == "" {
		identifier = "<unnamed>"
	}
	return file + "#" + identifier
}

// CanonicalFile retains the exact bytes of one indexed, normalized master
// file for pass-through consumers such as Gear Planner.
type CanonicalFile struct {
	MasterFile
	Data []byte
}

// Master is the canonical contract produced by the master generator and
// consumed directly by every domain generator.
type Master struct {
	Index     MasterIndex
	IndexData []byte
	Files     []CanonicalFile
	Items     []ItemRecord
	Augments  []AugmentRecord
}

// LoadMaster reconstructs the canonical contract from a generated master
// directory for independently invoked domain generation.
func LoadMaster(root string) (Master, error) {
	index, err := LoadMasterIndex(root)
	if err != nil {
		return Master{}, err
	}
	if index.SchemaVersion != 1 {
		return Master{}, fmt.Errorf("canonical master index schemaVersion must be 1")
	}
	indexData, err := os.ReadFile(filepath.Join(root, MasterIndexName))
	if err != nil {
		return Master{}, fmt.Errorf("read canonical master index: %w", err)
	}
	entries := append([]MasterFile(nil), index.Files...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	master := Master{Index: index, IndexData: indexData, Files: make([]CanonicalFile, 0, len(entries))}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if filepath.ToSlash(entry.Path) == MasterIndexName {
			return Master{}, fmt.Errorf("master index must not index itself")
		}
		path, err := secureJoin(root, filepath.FromSlash(entry.Path))
		if err != nil {
			return Master{}, fmt.Errorf("master entry %q: %w", entry.Path, err)
		}
		if _, exists := seen[entry.Path]; exists {
			return Master{}, fmt.Errorf("master index contains duplicate path %q", entry.Path)
		}
		seen[entry.Path] = struct{}{}
		data, err := os.ReadFile(path)
		if err != nil {
			return Master{}, fmt.Errorf("read canonical file %s: %w", entry.Path, err)
		}
		if !json.Valid(data) {
			return Master{}, fmt.Errorf("canonical file %s contains invalid JSON", entry.Path)
		}
		master.Files = append(master.Files, CanonicalFile{MasterFile: entry, Data: data})
		switch entry.Kind {
		case "items":
			var items []ItemData
			if err := json.Unmarshal(data, &items); err != nil {
				return Master{}, fmt.Errorf("decode %s: %w", path, err)
			}
			var rawItems []json.RawMessage
			if err := json.Unmarshal(data, &rawItems); err != nil {
				return Master{}, fmt.Errorf("decode raw items %s: %w", path, err)
			}
			if len(rawItems) != len(items) {
				return Master{}, fmt.Errorf("decode %s: typed and raw item counts differ", path)
			}
			for index, item := range items {
				master.Items = append(master.Items, ItemRecord{
					Category: entry.Category,
					File:     entry.Path,
					Item:     item,
					Raw:      append(json.RawMessage(nil), rawItems[index]...),
				})
			}
		case "augments":
			var augments []AugmentItem
			if err := ReadJSON(path, &augments); err != nil {
				return Master{}, err
			}
			for _, augment := range augments {
				master.Augments = append(master.Augments, AugmentRecord{Category: entry.Category, File: entry.Path, Augment: augment})
			}
		}
	}
	return master, nil
}
