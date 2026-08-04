package manual

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadItemSetNames(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ItemSetEnchantmentsFile)
	if err := os.WriteFile(path, []byte(`[
		{"name":"Later","bonuses":[]},
		{"name":"Earlier","bonuses":[]}
	]`), 0o644); err != nil {
		t.Fatal(err)
	}
	names, found, err := LoadItemSetNames(path)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !reflect.DeepEqual(names, []string{"Earlier", "Later"}) {
		t.Fatalf("names = %v, found = %t", names, found)
	}
}

func TestLoadItemSetNamesAllowsMissingFileAndRejectsInvalidNames(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), ItemSetEnchantmentsFile)
	names, found, err := LoadItemSetNames(missing)
	if err != nil || found || len(names) != 0 {
		t.Fatalf("missing result = %v, %t, %v", names, found, err)
	}

	for _, test := range []struct {
		name string
		data string
		want string
	}{
		{name: "empty", data: `[{"name":" ","bonuses":[]}]`, want: "empty name"},
		{name: "duplicate", data: `[{"name":"Set","bonuses":[]},{"name":"Set","bonuses":[]}]`, want: "duplicated"},
		{name: "malformed", data: `{}`, want: "cannot unmarshal"},
		{name: "null", data: `null`, want: "must be a JSON array"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ItemSetEnchantmentsFile)
			if err := os.WriteFile(path, []byte(test.data), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, err := LoadItemSetNames(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
