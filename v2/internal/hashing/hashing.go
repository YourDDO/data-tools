package hashing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func File(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func Directory(root string) (string, error) {
	paths, err := Files(root)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, relative := range paths {
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		fileHash, size, err := File(absolute)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s\x00%d\x00%s\x00", relative, size, fileHash)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Combine creates a stable digest from already-normalized labeled values.
func Combine(values ...string) string {
	hash := sha256.New()
	for _, value := range values {
		fmt.Fprintf(hash, "%d\x00%s\x00", len(value), value)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func Files(root string) ([]string, error) {
	result := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular file %s is not supported", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("file %s escapes root %s", path, root)
		}
		result = append(result, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list files under %s: %w", root, err)
	}
	sort.Strings(result)
	return result, nil
}
