package dataset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompactJSONMinifiesAndNormalizesTrailingNewline(t *testing.T) {
	t.Parallel()
	input := []byte("{\n  \"message\": \"spaces stay here\",\n  \"items\": [1, 2]\n}\n\n")
	got, err := CompactJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\"message\":\"spaces stay here\",\"items\":[1,2]}\n"; string(got) != want {
		t.Fatalf("compact JSON = %q, want %q", got, want)
	}
}

func TestCompactJSONRejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	if _, err := CompactJSON([]byte(`{"broken":`)); err == nil {
		t.Fatal("CompactJSON succeeded with invalid JSON")
	}
}

func TestWriteJSONWritesCompactBytes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "value.json")
	if err := WriteJSON(path, map[string]any{"b": []int{2, 1}, "a": true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\"a\":true,\"b\":[2,1]}\n"; string(data) != want {
		t.Fatalf("written JSON = %q, want %q", data, want)
	}
}
