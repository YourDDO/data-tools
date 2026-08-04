package itemlist

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"yourddo-data-tools/internal/dataset"
)

func TestGenerateCopiesExactCanonicalItem(t *testing.T) {
	t.Parallel()
	want := json.RawMessage(`{"pageTitle": "Complete Item", "name":"Complete Item","type":"Trinket","artifactType":"Minor","artifact":{"tier":"mythic"},"setBonus":[{"name":"Complete Set"}],"futureField":"<must&survive>","enchantments":[{"name":"Domain Marker"}]}`)
	var typed dataset.ItemData
	if err := json.Unmarshal(want, &typed); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	_, err := New("complete-items", func(dataset.ItemData) bool { return true }).Generate(
		context.Background(),
		dataset.Master{Items: []dataset.ItemRecord{{File: "trinkets.json", Item: typed, Raw: want}}},
		root,
	)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "complete-items", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got []json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !bytes.Equal(got[0], want) {
		t.Fatalf("generated item = %s, want exact master item %s", got[0], want)
	}
}
