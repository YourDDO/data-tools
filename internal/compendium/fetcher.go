package compendium

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/sys/unix"

	"yourddo-data-tools/internal/compendium/parser"
	"yourddo-data-tools/internal/dataset"
)

var aggregateCategories = map[string][]string{
	"Armor":     {"Docent", "Heavy Armor", "Medium Armor", "Light Armor", "Robe", "Outfit"},
	"Clothing":  {"Cloak", "Boots", "Gloves", "Helmet", "Belt"},
	"Exotic":    {"Bastard Sword", "Dwarven War Axe", "Great Crossbow", "Handwraps", "Kama", "Khopesh", "Repeating Heavy Crossbow", "Repeating Light Crossbow"},
	"Jewelry":   {"Goggles", "Ring", "Necklace", "Trinket", "Bracers"},
	"Martial":   {"Battle Axe", "Falchion", "Great Axe", "Great Club", "Great Sword", "Hand Axe", "Heavy Pick", "Kukri", "Light Hammer", "Light Pick", "Long Bow", "Long Sword", "Maul", "Rapier", "Scimitar", "Short Bow", "Short Sword", "Warhammer"},
	"Quiver":    {"Quiver"},
	"Shield":    {"Buckler", "Large Shield", "Orb", "Small Shield", "Tower Shield"},
	"Simple":    {"Club", "Dagger", "Light Crossbow", "Light Mace", "Heavy Crossbow", "Heavy Mace", "Morningstar", "Quarterstaff", "Sickle"},
	"Throwing":  {"Dart", "Shuriken", "Throwing Axe", "Throwing Dagger", "Throwing Hammer"},
	"Filigrees": {"Filigrees", "Filigree Sets"},
}

var standaloneSourceCategories = []string{"Augment", "Collar", "Rune Arm"}

// Result describes a complete canonical master dataset. SHA256 is a stable
// digest over the relative paths, byte sizes, and hashes of every output file.
type Result struct {
	Master     dataset.Master
	SHA256     string
	OutputRoot string
}

// Generator is the sole Compendium-aware stage. Domain generators consume its
// canonical files and never receive this source dependency.
type Generator struct {
	source Source
}

func NewGenerator(source Source) (*Generator, error) {
	if source == nil {
		return nil, fmt.Errorf("Compendium source is required")
	}
	return &Generator{source: source}, nil
}

// Fetcher and NewFetcher remain as compatibility names for callers of the
// initial v2 pipeline implementation.
type Fetcher = Generator

func NewFetcher(source Source) (*Generator, error) { return NewGenerator(source) }

// Fetch writes a create-only canonical dataset and returns its index. New code
// should use Generate so the master hash is available to the caller.
func (g *Generator) Fetch(ctx context.Context, categories []string, outputRoot string) (dataset.MasterIndex, error) {
	result, err := g.Generate(ctx, categories, outputRoot)
	return result.Master.Index, err
}

// Generate retrieves, parses, normalizes, and serializes a canonical master
// dataset in a temporary sibling directory. Promotion is create-only, so an
// existing dataset or published release can never be overwritten.
func (g *Generator) Generate(ctx context.Context, categories []string, outputRoot string) (result Result, returnErr error) {
	return g.generate(ctx, categories, outputRoot, false)
}

// GenerateReplacing is intended for local working datasets. It fully builds
// and hashes the replacement in a temporary sibling directory before swapping
// it into place. Immutable pipeline and release paths must use Generate.
func (g *Generator) GenerateReplacing(ctx context.Context, categories []string, outputRoot string) (result Result, returnErr error) {
	return g.generate(ctx, categories, outputRoot, true)
}

func (g *Generator) generate(ctx context.Context, categories []string, outputRoot string, replaceExisting bool) (result Result, returnErr error) {
	if len(categories) == 0 {
		return Result{}, fmt.Errorf("at least one Compendium category is required")
	}
	outputRoot = filepath.Clean(strings.TrimSpace(outputRoot))
	if outputRoot == "." || outputRoot == "" {
		return Result{}, fmt.Errorf("canonical output directory is required")
	}
	outputExists := false
	if info, err := os.Lstat(outputRoot); err == nil {
		outputExists = true
		if !info.IsDir() {
			return Result{}, fmt.Errorf("canonical output %s is not a directory", outputRoot)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("inspect canonical output %s: %w", outputRoot, err)
	}
	if outputExists && !replaceExisting {
		return Result{}, fmt.Errorf("canonical output %s already exists; refusing to overwrite it", outputRoot)
	}
	parent := filepath.Dir(outputRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Result{}, fmt.Errorf("create canonical output parent %s: %w", parent, err)
	}
	working, err := os.MkdirTemp(parent, ".master-work-*")
	if err != nil {
		return Result{}, fmt.Errorf("create temporary master working directory: %w", err)
	}
	defer func() {
		if working == "" {
			return
		}
		if err := os.RemoveAll(working); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("remove temporary master working directory: %w", err)
		}
	}()

	expanded := expandCategories(categories)
	if len(expanded) == 0 {
		return Result{}, fmt.Errorf("at least one non-empty Compendium category is required")
	}
	sort.Slice(expanded, func(i, j int) bool { return dataset.NaturalLess(expanded[i], expanded[j]) })
	index := dataset.MasterIndex{SchemaVersion: 1, Files: make([]dataset.MasterFile, 0, len(expanded))}
	written := make(map[string]writtenFile, len(expanded)+1)
	if replaceExisting && outputExists {
		copied, err := copyExistingOutput(outputRoot, working)
		if err != nil {
			return Result{}, err
		}
		for _, entry := range copied {
			written[entry.path] = entry
		}
		existing, err := dataset.LoadMasterIndex(outputRoot)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Result{}, fmt.Errorf("load existing local master index: %w", err)
		}
		if err == nil {
			if existing.SchemaVersion != 1 {
				return Result{}, fmt.Errorf("existing local master index schemaVersion must be 1")
			}
			index = existing
		}
	}
	identifiers := make(map[string]string)
	paths := make(map[string]string)
	for _, existing := range index.Files {
		paths[existing.Path] = existing.Category
	}

	for _, category := range expanded {
		contents, err := g.fetchRaw(ctx, category)
		if err != nil {
			return Result{}, err
		}
		kind, value, records, err := normalize(category, contents)
		if err != nil {
			return Result{}, err
		}
		for _, record := range records {
			key := canonicalKey(kind, record.identifier)
			if previous, exists := identifiers[key]; exists {
				return Result{}, fmt.Errorf("duplicate canonical identifier %q from source record %q; first seen at %q", key, record.source, previous)
			}
			identifiers[key] = record.source
		}
		relative := camelCase(category) + ".json"
		if previous, exists := paths[relative]; exists && !strings.EqualFold(previous, category) {
			return Result{}, fmt.Errorf("categories %q and %q produce duplicate canonical path %q", previous, category, relative)
		}
		paths[relative] = category
		if err := removeWorkingFile(working, relative); err != nil {
			return Result{}, fmt.Errorf("prepare category %q output: %w", category, err)
		}
		entry, err := writeCanonicalJSON(filepath.Join(working, filepath.FromSlash(relative)), value, false)
		if err != nil {
			return Result{}, fmt.Errorf("write category %q: %w", category, err)
		}
		entry.path = relative
		written[relative] = entry
		upsertMasterFile(&index, dataset.MasterFile{Category: category, Kind: kind, Path: relative})
	}

	sort.Slice(index.Files, func(i, j int) bool {
		if index.Files[i].Kind != index.Files[j].Kind {
			return index.Files[i].Kind < index.Files[j].Kind
		}
		return index.Files[i].Path < index.Files[j].Path
	})
	if err := removeWorkingFile(working, dataset.MasterIndexName); err != nil {
		return Result{}, fmt.Errorf("prepare master index output: %w", err)
	}
	indexEntry, err := writeCanonicalJSON(filepath.Join(working, dataset.MasterIndexName), index, true)
	if err != nil {
		return Result{}, fmt.Errorf("write master index: %w", err)
	}
	indexEntry.path = dataset.MasterIndexName
	written[dataset.MasterIndexName] = indexEntry
	masterHash := hashWrittenFiles(mapValues(written))

	promotionFlag := uint(unix.RENAME_NOREPLACE)
	if replaceExisting && outputExists {
		promotionFlag = unix.RENAME_EXCHANGE
	}
	if err := unix.Renameat2(unix.AT_FDCWD, working, unix.AT_FDCWD, outputRoot, promotionFlag); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return Result{}, fmt.Errorf("canonical output %s already exists; refusing to overwrite it", outputRoot)
		}
		return Result{}, fmt.Errorf("promote canonical output to %s: %w", outputRoot, err)
	}
	if promotionFlag != unix.RENAME_EXCHANGE {
		working = ""
	}
	master, err := dataset.LoadMaster(outputRoot)
	if err != nil {
		return Result{}, fmt.Errorf("load generated canonical master dataset: %w", err)
	}
	return Result{Master: master, SHA256: masterHash, OutputRoot: outputRoot}, nil
}

func (g *Generator) fetchRaw(ctx context.Context, category string) (map[string]string, error) {
	if strings.EqualFold(category, "Filigree Sets") {
		const page = "Filigree_Item_Sets"
		content, err := g.source.FetchPageContent(ctx, page)
		if err != nil {
			return nil, fmt.Errorf("fetch category %q source record %q: %w", category, page, err)
		}
		if strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("fetch category %q source record %q: empty content", category, page)
		}
		return map[string]string{page: content}, nil
	}
	content, err := g.source.FetchCategoryContent(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("fetch category %q: %w", category, err)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("fetch category %q: no source records returned", category)
	}
	return content, nil
}

type canonicalRecord struct {
	identifier string
	source     string
}

func normalize(category string, contents map[string]string) (string, any, []canonicalRecord, error) {
	titles := make([]string, 0, len(contents))
	for title := range contents {
		titles = append(titles, title)
	}
	sort.Slice(titles, func(i, j int) bool { return dataset.NaturalLess(titles[i], titles[j]) })
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "augment", "augments":
		augments := make([]dataset.AugmentItem, 0, len(titles))
		records := make([]canonicalRecord, 0, len(titles))
		for _, title := range titles {
			if parser.MasterExclusionReason(title, contents[title]) != "" {
				continue
			}
			augment, err := parser.ParseAugmentRecord(title, contents[title])
			if err != nil {
				return "", nil, nil, fmt.Errorf("parse category %q source record %q: %w", category, title, err)
			}
			if parser.HasDiscontinuedDropLocation(contents[title]) {
				continue
			}
			normalizeAugment(&augment)
			augments = append(augments, augment)
			records = append(records, canonicalRecord{identifier: title, source: category + "/" + title})
		}
		sort.Slice(augments, func(i, j int) bool {
			if augments[i].Name != augments[j].Name {
				return dataset.NaturalLess(augments[i].Name, augments[j].Name)
			}
			return jsonKey(augments[i]) < jsonKey(augments[j])
		})
		return "augments", augments, records, nil
	case "filigree sets":
		sets := make([]dataset.FiligreeSet, 0)
		records := make([]canonicalRecord, 0)
		for _, title := range titles {
			parsed, err := parser.ParseFiligreeSetRecords(contents[title])
			if err != nil {
				return "", nil, nil, fmt.Errorf("parse category %q source record %q: %w", category, title, err)
			}
			for index := range parsed {
				normalizeFiligreeSet(&parsed[index])
				sets = append(sets, parsed[index])
				records = append(records, canonicalRecord{identifier: parsed[index].Name, source: category + "/" + title + "#" + parsed[index].Name})
			}
		}
		sort.Slice(sets, func(i, j int) bool {
			if sets[i].Name != sets[j].Name {
				return dataset.NaturalLess(sets[i].Name, sets[j].Name)
			}
			return jsonKey(sets[i]) < jsonKey(sets[j])
		})
		return "filigree-sets", sets, records, nil
	default:
		items := make([]dataset.ItemData, 0, len(titles))
		records := make([]canonicalRecord, 0, len(titles))
		for _, title := range titles {
			if parser.MasterExclusionReason(title, contents[title]) != "" {
				continue
			}
			item, err := parser.ParseItemRecord(title, contents[title])
			if err != nil {
				return "", nil, nil, fmt.Errorf("parse category %q source record %q: %w", category, title, err)
			}
			if parser.HasDiscontinuedDropLocation(contents[title]) {
				continue
			}
			normalizeItem(&item)
			items = append(items, item)
			records = append(records, canonicalRecord{identifier: item.PageTitle, source: category + "/" + title})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Name != items[j].Name {
				return dataset.NaturalLess(items[i].Name, items[j].Name)
			}
			return items[i].PageTitle < items[j].PageTitle
		})
		return "items", items, records, nil
	}
}

func canonicalKey(kind, identifier string) string {
	return strings.ToLower(strings.TrimSpace(kind)) + ":" + strings.ToLower(strings.TrimSpace(identifier))
}

// normalizeItem preserves enchantment order because it represents the source
// presentation, while sorting set-like nested collections.
func normalizeItem(item *dataset.ItemData) {
	for index := range item.DropLocations {
		sort.Strings(item.DropLocations[index].RaidItems)
		sort.Strings(item.DropLocations[index].VeteranClasses)
		sortCraftingRequirements(item.DropLocations[index].Ingredients)
	}
	sort.Slice(item.DropLocations, func(i, j int) bool { return jsonKey(item.DropLocations[i]) < jsonKey(item.DropLocations[j]) })
	sortCraftingRequirements(item.CraftingRequirements)
	for index := range item.Augments {
		normalizeAugment(&item.Augments[index])
	}
	sort.Slice(item.Augments, func(i, j int) bool { return jsonKey(item.Augments[i]) < jsonKey(item.Augments[j]) })
	for index := range item.SetBonus {
		sortEnhancements(item.SetBonus[index].Enhancements)
	}
	sort.Slice(item.SetBonus, func(i, j int) bool { return jsonKey(item.SetBonus[i]) < jsonKey(item.SetBonus[j]) })
}

func normalizeAugment(augment *dataset.AugmentItem) {
	augment.FoundIn = sortedUnique(augment.FoundIn)
	for index := range augment.Requirements {
		normalizeAugment(&augment.Requirements[index])
	}
	sort.Slice(augment.Requirements, func(i, j int) bool { return jsonKey(augment.Requirements[i]) < jsonKey(augment.Requirements[j]) })
	for _, values := range []*[]dataset.PartialEnhancementOut{
		&augment.Enhancements, &augment.AccessoryEffectsAdded, &augment.AccessoryEffectsRemoved,
		&augment.EffectsAdded, &augment.EffectsRemoved, &augment.WeaponEffectsAdded, &augment.WeaponEffectsRemoved,
	} {
		sortEnhancements(*values)
	}
	for index := range augment.SetBonus {
		sortEnhancements(augment.SetBonus[index].Enhancements)
	}
	sort.Slice(augment.SetBonus, func(i, j int) bool { return jsonKey(augment.SetBonus[i]) < jsonKey(augment.SetBonus[j]) })
	if augment.Spell != nil {
		augment.Spell.Target = sortedUnique(augment.Spell.Target)
	}
	if augment.RuneArmBlast != nil {
		augment.RuneArmBlast.Target = sortedUnique(augment.RuneArmBlast.Target)
	}
}

func normalizeFiligreeSet(set *dataset.FiligreeSet) {
	for index := range set.Bonuses {
		sortEnhancements(set.Bonuses[index].Enhancements)
	}
	sort.Slice(set.Bonuses, func(i, j int) bool {
		if set.Bonuses[i].Threshold != set.Bonuses[j].Threshold {
			return set.Bonuses[i].Threshold < set.Bonuses[j].Threshold
		}
		return jsonKey(set.Bonuses[i]) < jsonKey(set.Bonuses[j])
	})
}

func sortCraftingRequirements(values []dataset.CraftingRequirement) {
	for index := range values {
		values[index].Location = sortedUnique(values[index].Location)
	}
	sort.Slice(values, func(i, j int) bool { return jsonKey(values[i]) < jsonKey(values[j]) })
}

func sortEnhancements(values []dataset.PartialEnhancementOut) {
	for index := range values {
		values[index].Damage = sortedUnique(values[index].Damage)
	}
	sort.Slice(values, func(i, j int) bool { return jsonKey(values[i]) < jsonKey(values[j]) })
}

func sortedUnique(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	write := 1
	for read := 1; read < len(result); read++ {
		if result[read] != result[write-1] {
			result[write] = result[read]
			write++
		}
	}
	return result[:write]
}

func jsonKey(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

type writtenFile struct {
	path   string
	size   int64
	sha256 string
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)
	w.n += int64(n)
	return n, err
}

func writeCanonicalJSON(path string, value any, indent bool) (entry writtenFile, returnErr error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return writtenFile{}, fmt.Errorf("create parent for %s: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return writtenFile{}, fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("close %s: %w", path, err)
		}
	}()
	digest := sha256.New()
	counter := &countingWriter{w: io.MultiWriter(file, digest)}
	encoder := json.NewEncoder(counter)
	if indent {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		return writtenFile{}, fmt.Errorf("encode %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return writtenFile{}, fmt.Errorf("sync %s: %w", path, err)
	}
	return writtenFile{size: counter.n, sha256: hex.EncodeToString(digest.Sum(nil))}, nil
}

func hashWrittenFiles(files []writtenFile) string {
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	digest := sha256.New()
	for _, file := range files {
		writeFileIdentity(digest, file)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func writeFileIdentity(destination hash.Hash, file writtenFile) {
	fmt.Fprintf(destination, "%s\x00%d\x00%s\x00", file.path, file.size, file.sha256)
}

func copyExistingOutput(sourceRoot, destinationRoot string) ([]writtenFile, error) {
	entries := make([]writtenFile, 0)
	err := filepath.WalkDir(sourceRoot, func(path string, directoryEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		destination := filepath.Join(destinationRoot, relative)
		if directoryEntry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		info, err := directoryEntry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("existing local output contains unsupported non-regular file %s", path)
		}
		entry, err := copyAndHashFile(path, destination, info.Mode().Perm())
		if err != nil {
			return err
		}
		entry.path = filepath.ToSlash(relative)
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("stage existing local canonical output: %w", err)
	}
	return entries, nil
}

func copyAndHashFile(sourcePath, destinationPath string, mode os.FileMode) (writtenFile, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return writtenFile{}, fmt.Errorf("open existing file %s: %w", sourcePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		source.Close()
		return writtenFile{}, fmt.Errorf("create parent for copied file %s: %w", destinationPath, err)
	}
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		source.Close()
		return writtenFile{}, fmt.Errorf("create copied file %s: %w", destinationPath, err)
	}
	digest := sha256.New()
	counter := &countingWriter{w: io.MultiWriter(destination, digest)}
	_, copyErr := io.Copy(counter, source)
	sourceCloseErr := source.Close()
	syncErr := destination.Sync()
	destinationCloseErr := destination.Close()
	if copyErr != nil {
		return writtenFile{}, fmt.Errorf("copy existing file %s: %w", sourcePath, copyErr)
	}
	if sourceCloseErr != nil {
		return writtenFile{}, fmt.Errorf("close existing file %s: %w", sourcePath, sourceCloseErr)
	}
	if syncErr != nil {
		return writtenFile{}, fmt.Errorf("sync copied file %s: %w", destinationPath, syncErr)
	}
	if destinationCloseErr != nil {
		return writtenFile{}, fmt.Errorf("close copied file %s: %w", destinationPath, destinationCloseErr)
	}
	return writtenFile{size: counter.n, sha256: hex.EncodeToString(digest.Sum(nil))}, nil
}

func removeWorkingFile(root, relative string) error {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", relative)
	}
	return os.Remove(path)
}

func upsertMasterFile(index *dataset.MasterIndex, replacement dataset.MasterFile) {
	files := make([]dataset.MasterFile, 0, len(index.Files)+1)
	for _, existing := range index.Files {
		if existing.Path == replacement.Path ||
			(strings.EqualFold(existing.Category, replacement.Category) && existing.Kind == replacement.Kind) {
			continue
		}
		files = append(files, existing)
	}
	index.Files = append(files, replacement)
}

func mapValues(values map[string]writtenFile) []writtenFile {
	result := make([]writtenFile, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func expandCategories(categories []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, requested := range categories {
		requested = strings.TrimSpace(requested)
		if requested == "" {
			continue
		}
		values := []string{requested}
		if strings.EqualFold(requested, "All") {
			values = allSourceCategories()
		} else if aggregate, ok := aggregateCategories[requested]; ok {
			values = aggregate
		}
		for _, value := range values {
			key := strings.ToLower(value)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func allSourceCategories() []string {
	groups := make([]string, 0, len(aggregateCategories))
	for group := range aggregateCategories {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	categories := append([]string(nil), standaloneSourceCategories...)
	for _, group := range groups {
		categories = append(categories, aggregateCategories[group]...)
	}
	sort.Slice(categories, func(i, j int) bool {
		return dataset.NaturalLess(categories[i], categories[j])
	})
	return categories
}

func camelCase(value string) string {
	words := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		return unicode.IsSpace(r) || r == '_' || r == '-'
	})
	for index := range words {
		words[index] = strings.ToLower(words[index])
		if index > 0 && words[index] != "" {
			runes := []rune(words[index])
			runes[0] = unicode.ToUpper(runes[0])
			words[index] = string(runes)
		}
	}
	return strings.Join(words, "")
}
