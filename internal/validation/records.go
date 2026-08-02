package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"yourddo-data-tools/internal/contracts"
	"yourddo-data-tools/internal/dataset"
)

var leadingInteger = regexp.MustCompile(`^-?\d+`)

type masterIDs struct {
	pages    map[string]struct{}
	names    map[string]struct{}
	augments map[string]struct{}
}

func validateMasterRecords(root string, index dataset.MasterIndex) Report {
	var report Report
	seen := map[string]map[string]string{
		"items":         {},
		"augments":      {},
		"filigree-sets": {},
	}
	for _, entry := range index.Files {
		cleaned, ok := cleanRelative(entry.Path)
		if !ok {
			continue
		}
		path := filepath.Join(root, cleaned)
		records, valid := decodeRecordArray(path)
		if !valid {
			continue
		}
		if len(records) == 0 {
			report.add(Error, "master", entry.Path, "<file>", "non-empty-dataset", "indexed master dataset is unexpectedly empty")
		}
		for position, record := range records {
			recordID := recordIdentifier(record, position)
			var identifier string
			switch entry.Kind {
			case "items":
				identifier = stringField(record, "pageTitle")
				requireFields(&report, "master", entry.Path, recordID, record, "pageTitle", "name")
				validateLevel(&report, "master", entry.Path, recordID, record["minLevel"])
			case "augments":
				identifier = canonicalRecordIdentifier(record)
				requireFields(&report, "master", entry.Path, recordID, record, "name")
				validateLevel(&report, "master", entry.Path, recordID, record["minLevel"])
			case "filigree-sets":
				identifier = stringField(record, "name")
				requireFields(&report, "master", entry.Path, recordID, record, "name", "bonuses")
			}
			if identifier == "" {
				continue
			}
			if previous, exists := seen[entry.Kind][identifier]; exists {
				report.add(Error, "master", entry.Path, recordID, "unique-identifier", fmt.Sprintf("identifier is already present in %s", previous))
			} else {
				seen[entry.Kind][identifier] = entry.Path
			}
		}
	}
	return report
}

// Augment names and slot types are not unique in the upstream contract. The
// canonical record itself is therefore the only available identifier for
// detecting a duplicated augment without rejecting legitimate variants.
func canonicalRecordIdentifier(record map[string]any) string {
	data, err := json.Marshal(record)
	if err != nil {
		return ""
	}
	return string(data)
}

func validateRecordFiles(root string, files []contracts.GeneratedFileMetadata, master *dataset.Master) Report {
	var report Report
	ids := buildMasterIDs(master)
	effects := collectIdentifiers(root, files, "effects.json", "id")
	plannerEntries := collectIdentifiers(root, files, "planner_entries.json", "id")
	for _, file := range files {
		if strings.HasPrefix(filepath.ToSlash(file.Path), "master/") || filepath.Base(file.Path) == dataset.MasterIndexName {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		base := filepath.Base(file.Path)
		if base == "setBonusIndex.json" {
			report.merge(validateSetBonusIndex(path, file, ids))
			continue
		}
		if base == "indexes.json" {
			report.merge(validateEssenceIndexes(path, file, effects, plannerEntries))
			continue
		}
		records, valid := decodeRecordArray(path)
		if !valid {
			continue
		}
		if expectedNonEmpty(base, records) && len(records) == 0 {
			report.add(Error, file.Domain, file.Path, "<file>", "non-empty-dataset", "generated dataset is unexpectedly empty")
		}
		seen := make(map[string]int)
		for position, record := range records {
			recordID := recordIdentifier(record, position)
			identifier := ""
			switch {
			case base == "items.json":
				identifier = stringField(record, "pageTitle")
				requireFields(&report, file.Domain, file.Path, recordID, record, "pageTitle", "name")
				requirePresent(&report, file.Domain, file.Path, recordID, record, "type", "minLevel", "enchantments")
				validateLevel(&report, file.Domain, file.Path, recordID, record["minLevel"])
				if master != nil && identifier != "" {
					if _, exists := ids.pages[identifier]; !exists {
						report.add(Error, file.Domain, file.Path, identifier, "master-reference", "item pageTitle does not reference a master item")
					}
				}
			case stringField(record, "pageTitle") != "":
				identifier = stringField(record, "pageTitle")
				requireFields(&report, file.Domain, file.Path, recordID, record, "pageTitle", "name")
				validateLevel(&report, file.Domain, file.Path, recordID, record["minLevel"])
				if master != nil && identifier != "" {
					if _, exists := ids.pages[identifier]; !exists {
						report.add(Error, file.Domain, file.Path, identifier, "master-reference", "item pageTitle does not reference a master item")
					}
				}
			case base == "upgrades.json":
				identifier = stringField(record, "name")
				requireFields(&report, file.Domain, file.Path, recordID, record, "name")
				if _, removed := record["effectsRemoved"]; !removed {
					if _, added := record["effectsAdded"]; !added {
						report.add(Error, file.Domain, file.Path, recordID, "required-fields", "at least one effectsRemoved or effectsAdded field is required")
					}
				}
				if master != nil && identifier != "" {
					if _, exists := ids.names[identifier]; !exists {
						report.add(Error, file.Domain, file.Path, identifier, "master-reference", "upgrade name does not reference a master item")
					}
				}
			case base == "augments.json":
				identifier = stringField(record, "name")
				requireFields(&report, file.Domain, file.Path, recordID, record, "name", "augmentType")
				validateLevel(&report, file.Domain, file.Path, recordID, record["minLevel"])
				validatePositiveQuantities(&report, file.Domain, file.Path, recordID, record["requirements"])
				if master != nil && identifier != "" {
					if _, exists := ids.augments[identifier]; !exists {
						report.add(Error, file.Domain, file.Path, identifier, "master-reference", "augment name does not reference a master augment")
					}
				}
			case base == "effects.json":
				identifier = stringField(record, "id")
				requireFields(&report, file.Domain, file.Path, recordID, record, "id", "name")
			case base == "planner_entries.json":
				identifier = stringField(record, "id")
				requireFields(&report, file.Domain, file.Path, recordID, record, "id", "sourceType", "effectId", "enchantmentName", "slotId", "affixType")
				validateEnum(&report, file.Domain, file.Path, recordID, "sourceType", stringField(record, "sourceType"), "essence")
				validateEnum(&report, file.Domain, file.Path, recordID, "affixType", stringField(record, "affixType"), "prefix", "suffix", "extra")
				validateReference(&report, file, recordID, "effectId", stringField(record, "effectId"), effects)
			case base == "enchantments.json" || base == "placements.json" || base == "tiers.json" || base == "recipes.json" || base == "display.json":
				identifier = stringField(record, "effectId")
				requireFields(&report, file.Domain, file.Path, recordID, record, "effectId")
				validateReference(&report, file, recordID, "effectId", identifier, effects)
				if base == "recipes.json" {
					validateRecipe(&report, file, recordID, record)
				}
				if base == "tiers.json" {
					validateTiers(&report, file, recordID, record["tiers"])
				}
			case base == "shards.json":
				identifier = firstNonEmpty(stringField(record, "id"), stringField(record, "name"))
				if identifier == "" {
					report.add(Error, file.Domain, file.Path, recordID, "required-fields", "id or name is required")
				}
			}
			if identifier != "" {
				if previous, exists := seen[identifier]; exists {
					report.add(Error, file.Domain, file.Path, identifier, "unique-identifier", fmt.Sprintf("identifier duplicates record at index %d", previous))
				} else {
					seen[identifier] = position
				}
			}
		}
	}
	return report
}

func validateSetBonusIndex(path string, file contracts.GeneratedFileMetadata, ids masterIDs) Report {
	var report Report
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		return report
	}
	var sets map[string][]map[string]any
	if err := decodeStrict(data, &sets); err != nil {
		report.add(Error, file.Domain, file.Path, "<file>", "valid-schema", err.Error())
		return report
	}
	for setName, records := range sets {
		if strings.TrimSpace(setName) == "" {
			report.add(Error, file.Domain, file.Path, "<empty>", "required-fields", "set name is required")
		}
		seen := make(map[string]struct{})
		for position, record := range records {
			name := stringField(record, "name")
			recordID := setName + "/" + firstNonEmpty(name, strconv.Itoa(position))
			requireFields(&report, file.Domain, file.Path, recordID, record, "name", "minLevel")
			validateLevel(&report, file.Domain, file.Path, recordID, record["minLevel"])
			if _, exists := seen[name]; exists && name != "" {
				report.add(Error, file.Domain, file.Path, recordID, "duplicate-item", "item appears more than once in the set")
			}
			seen[name] = struct{}{}
			if len(ids.pages) != 0 {
				_, pageExists := ids.pages[name]
				_, nameExists := ids.names[name]
				if !pageExists && !nameExists {
					report.add(Error, file.Domain, file.Path, recordID, "master-reference", "set item does not reference a master item")
				}
			}
		}
	}
	return report
}

func validateEssenceIndexes(path string, file contracts.GeneratedFileMetadata, effects, plannerEntries map[string]struct{}) Report {
	var report Report
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		return report
	}
	var indexes map[string]map[string][]string
	if err := decodeStrict(data, &indexes); err != nil {
		report.add(Error, file.Domain, file.Path, "<file>", "valid-schema", err.Error())
		return report
	}
	if len(indexes) == 0 {
		report.add(Error, file.Domain, file.Path, "<file>", "non-empty-dataset", "index dataset is unexpectedly empty")
	}
	for indexName, values := range indexes {
		valid := effects
		if strings.HasPrefix(indexName, "plannerEntryIds") {
			valid = plannerEntries
		}
		for key, identifiers := range values {
			seen := make(map[string]struct{})
			for _, identifier := range identifiers {
				recordID := indexName + "/" + key
				if _, exists := seen[identifier]; exists {
					report.add(Error, file.Domain, file.Path, recordID, "unique-identifier", fmt.Sprintf("index contains duplicate identifier %q", identifier))
				}
				seen[identifier] = struct{}{}
				if _, exists := valid[identifier]; !exists {
					report.add(Error, file.Domain, file.Path, recordID, "referential-integrity", fmt.Sprintf("index references unknown identifier %q", identifier))
				}
			}
		}
	}
	return report
}

func validateCountDrift(root string, files []contracts.GeneratedFileMetadata, options Options) Report {
	var report Report
	baselineFiles := make(map[string]contracts.GeneratedFileMetadata, len(options.Baseline.GeneratedFiles))
	for _, file := range options.Baseline.GeneratedFiles {
		baselineFiles[file.Path] = file
	}
	for _, file := range files {
		if _, exists := baselineFiles[file.Path]; !exists {
			continue
		}
		oldCount, oldOK := recordCount(filepath.Join(options.BaselineRoot, filepath.FromSlash(file.Path)))
		newCount, newOK := recordCount(filepath.Join(root, filepath.FromSlash(file.Path)))
		if !oldOK || !newOK || oldCount < minimumDriftBaseline {
			continue
		}
		change := float64(newCount-oldCount) / float64(oldCount)
		if change < -options.MaximumReduction {
			report.add(Error, file.Domain, file.Path, "<file>", "record-count-reduction", fmt.Sprintf("record count fell from %d to %d (%.1f%%)", oldCount, newCount, -change*100))
		} else if change > options.MaximumIncrease {
			report.add(Warning, file.Domain, file.Path, "<file>", "record-count-increase", fmt.Sprintf("record count rose from %d to %d (%.1f%%)", oldCount, newCount, change*100))
		}
	}
	return report
}

func buildMasterIDs(master *dataset.Master) masterIDs {
	result := masterIDs{pages: map[string]struct{}{}, names: map[string]struct{}{}, augments: map[string]struct{}{}}
	if master == nil {
		return result
	}
	for _, record := range master.Items {
		result.pages[record.Item.PageTitle] = struct{}{}
		result.names[record.Item.Name] = struct{}{}
	}
	for _, record := range master.Augments {
		result.augments[record.Augment.Name] = struct{}{}
	}
	return result
}

func collectIdentifiers(root string, files []contracts.GeneratedFileMetadata, basename, field string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, file := range files {
		if filepath.Base(file.Path) != basename {
			continue
		}
		records, valid := decodeRecordArray(filepath.Join(root, filepath.FromSlash(file.Path)))
		if !valid {
			continue
		}
		for _, record := range records {
			if value := stringField(record, field); value != "" {
				result[value] = struct{}{}
			}
		}
	}
	return result
}

func decodeRecordArray(path string) ([]map[string]any, bool) {
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		return nil, false
	}
	var records []map[string]any
	if err := decodeStrict(data, &records); err != nil {
		return nil, false
	}
	return records, true
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return nil
}

func requireFields(report *Report, datasetName, file, recordID string, record map[string]any, fields ...string) {
	for _, field := range fields {
		value, exists := record[field]
		if !exists || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
			report.add(Error, datasetName, file, recordID, "required-fields", fmt.Sprintf("field %s is required", field))
		}
	}
}

func requirePresent(report *Report, datasetName, file, recordID string, record map[string]any, fields ...string) {
	for _, field := range fields {
		if _, exists := record[field]; !exists {
			report.add(Error, datasetName, file, recordID, "required-fields", fmt.Sprintf("field %s is required", field))
		}
	}
}

func validateLevel(report *Report, datasetName, file, recordID string, raw any) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		return
	}
	value, ok := integerValue(raw)
	if !ok {
		report.add(Error, datasetName, file, recordID, "valid-minimum-maximum", "minLevel must begin with an integer")
		return
	}
	if value < 0 || value > 100 {
		report.add(Error, datasetName, file, recordID, "valid-minimum-maximum", "minLevel must be between 0 and 100")
	}
}

func validatePositiveQuantities(report *Report, datasetName, file, recordID string, raw any) {
	values, ok := raw.([]any)
	if !ok {
		return
	}
	for position, value := range values {
		requirement, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if quantity, exists := requirement["quantity"]; exists {
			parsed, valid := integerValue(quantity)
			if !valid || parsed <= 0 || parsed > 1_000_000 {
				report.add(Error, datasetName, file, fmt.Sprintf("%s/requirements[%d]", recordID, position), "valid-minimum-maximum", "quantity must be between 1 and 1000000")
			}
		}
	}
}

func validateRecipe(report *Report, file contracts.GeneratedFileMetadata, recordID string, record map[string]any) {
	if _, bound := record["bound"]; !bound {
		if _, unbound := record["unbound"]; !unbound {
			report.add(Error, file.Domain, file.Path, recordID, "required-fields", "bound or unbound recipe data is required")
		}
	}
	for _, key := range []string{"bound", "unbound"} {
		cost, ok := record[key].(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []string{"level", "essence", "purified"} {
			if raw, exists := cost[field]; exists {
				value, valid := integerValue(raw)
				if !valid || value < 0 || value > 1_000_000 {
					report.add(Error, file.Domain, file.Path, recordID, "valid-minimum-maximum", fmt.Sprintf("%s.%s must be between 0 and 1000000", key, field))
				}
			}
		}
	}
}

func validateTiers(report *Report, file contracts.GeneratedFileMetadata, recordID string, raw any) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		report.add(Error, file.Domain, file.Path, recordID, "required-fields", "tiers must contain at least one tier")
		return
	}
	seen := make(map[int]struct{})
	for _, value := range values {
		tier, ok := value.(map[string]any)
		if !ok {
			continue
		}
		number, valid := integerValue(tier["tier"])
		if !valid || number < 1 || number > 100 {
			report.add(Error, file.Domain, file.Path, recordID, "valid-minimum-maximum", "tier must be between 1 and 100")
		}
		if _, exists := seen[number]; exists {
			report.add(Error, file.Domain, file.Path, recordID, "unique-identifier", fmt.Sprintf("tier %d is duplicated", number))
		}
		seen[number] = struct{}{}
	}
}

func validateEnum(report *Report, datasetName, file, recordID, field, value string, allowed ...string) {
	for _, candidate := range allowed {
		if value == candidate {
			return
		}
	}
	if value != "" {
		report.add(Error, datasetName, file, recordID, "valid-enum", fmt.Sprintf("%s %q is not one of %s", field, value, strings.Join(allowed, ", ")))
	}
}

func validateReference(report *Report, file contracts.GeneratedFileMetadata, recordID, field, value string, valid map[string]struct{}) {
	if value == "" {
		return
	}
	if _, exists := valid[value]; !exists {
		report.add(Error, file.Domain, file.Path, recordID, "referential-integrity", fmt.Sprintf("%s references unknown effect %q", field, value))
	}
}

func integerValue(raw any) (int, bool) {
	text := strings.TrimSpace(fmt.Sprint(raw))
	match := leadingInteger.FindString(text)
	if match == "" {
		return 0, false
	}
	value, err := strconv.Atoi(match)
	return value, err == nil
}

func stringField(record map[string]any, field string) string {
	value, ok := record[field].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func recordIdentifier(record map[string]any, position int) string {
	return firstNonEmpty(stringField(record, "id"), stringField(record, "pageTitle"), stringField(record, "name"), stringField(record, "effectId"), fmt.Sprintf("index:%d", position))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func expectedNonEmpty(base string, records []map[string]any) bool {
	switch base {
	case "items.json", "upgrades.json", "augments.json", "effects.json", "enchantments.json", "placements.json", "tiers.json", "recipes.json", "display.json", "planner_entries.json", "shards.json":
		return true
	default:
		return false
	}
}

func recordCount(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		return 0, false
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return 0, false
	}
	switch typed := value.(type) {
	case []any:
		return len(typed), true
	case map[string]any:
		return len(typed), true
	default:
		return 0, false
	}
}
