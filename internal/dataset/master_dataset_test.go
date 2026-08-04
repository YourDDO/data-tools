package dataset

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMasterRetainsExactRawItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := []byte(`{"pageTitle":"Raw Item","name":"Raw Item","artifactType":"Minor","future":{"value":"preserved"}}`)
	data := append(append([]byte{'['}, want...), ']', '\n')
	if err := os.WriteFile(filepath.Join(root, "items.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteMasterIndex(root, MasterIndex{
		SchemaVersion: 1,
		Files:         []MasterFile{{Category: "Items", Kind: "items", Path: "items.json"}},
	}); err != nil {
		t.Fatal(err)
	}

	master, err := LoadMaster(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(master.Items) != 1 || !bytes.Equal(master.Items[0].Raw, want) {
		t.Fatalf("raw item = %s, want %s", master.Items[0].Raw, want)
	}
}
