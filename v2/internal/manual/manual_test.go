package manual

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareDiscoversAndCanonicalizesDeterministically(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	destination := t.TempDir()
	writeTestFile(t, filepath.Join(source, "z.json"), `{"b":2,"a":[3,1]}`)
	writeTestFile(t, filepath.Join(source, "nested", "a.json"), "{\n  \"value\": true\n}\n")
	writeTestFile(t, filepath.Join(source, "README.md"), "ignored")

	payloads, err := Prepare(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if names := []string{payloads[0].Name, payloads[1].Name}; !reflect.DeepEqual(names, []string{"nested/a", "z"}) {
		t.Fatalf("logical names = %v", names)
	}
	want := "{\n  \"a\": [\n    3,\n    1\n  ],\n  \"b\": 2\n}\n"
	data, err := os.ReadFile(filepath.Join(destination, "z.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("canonical payload = %q, want %q", data, want)
	}
	digest := sha256.Sum256(data)
	if payloads[1].Path != "manual/z.json" || payloads[1].SHA256 != hex.EncodeToString(digest[:]) || payloads[1].SizeBytes != int64(len(data)) {
		t.Fatalf("metadata = %#v", payloads[1])
	}
}

func TestCanonicalizeIgnoresFormattingAndObjectOrderButPreservesArrayOrder(t *testing.T) {
	t.Parallel()
	first, err := Canonicalize([]byte(`{"b":{"y":2,"x":1},"a":[2,1]}`))
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := Canonicalize([]byte("{\n \"a\" : [2, 1], \"b\": {\"x\":1, \"y\":2}\n}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(formatted) {
		t.Fatalf("formatting changed canonical bytes:\n%s\n%s", first, formatted)
	}
	reordered, err := Canonicalize([]byte(`{"a":[1,2],"b":{"x":1,"y":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(reordered) {
		t.Fatal("array order did not affect canonical bytes")
	}
}

func TestPrepareRejectsMalformedPayloads(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		data string
		want string
	}{
		{name: "malformed", data: `{"a":`, want: "unexpected EOF"},
		{name: "trailing", data: `{} {}`, want: "trailing JSON value"},
		{name: "duplicate key", data: `{"a":1,"a":2}`, want: "duplicate object key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			writeTestFile(t, filepath.Join(source, "payload.json"), test.data)
			_, err := Prepare(source, t.TempDir())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPrepareAllowsMissingAndEmptyInputDirectory(t *testing.T) {
	t.Parallel()
	for _, source := range []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		payloads, err := Prepare(source, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(payloads) != 0 {
			t.Fatalf("payloads = %#v", payloads)
		}
	}
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
