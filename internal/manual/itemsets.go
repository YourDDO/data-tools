package manual

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"yourddo-data-tools/internal/dataset"
)

const ItemSetEnchantmentsFile = "itemSets.enchantments.json"

// LoadItemSetNames validates the maintained item-set payload and returns its
// exact, case-sensitive names. A missing payload is allowed so generic manual
// input directories remain supported.
func LoadItemSetNames(path string) (names []string, found bool, err error) {
	var sets []dataset.ItemSet
	if err := dataset.ReadJSON(path, &sets); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, false, nil
		}
		return nil, false, err
	}
	if sets == nil {
		return nil, true, fmt.Errorf("manual item-set definitions must be a JSON array")
	}
	seen := make(map[string]struct{}, len(sets))
	for index, set := range sets {
		name := strings.TrimSpace(set.Name)
		if name == "" {
			return nil, true, fmt.Errorf("item set at index %d has an empty name", index)
		}
		if _, exists := seen[name]; exists {
			return nil, true, fmt.Errorf("item set name %q is duplicated", name)
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, true, nil
}
