package publish

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	root string
}

func NewLocalStore(root string) (*LocalStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("local publication root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local publication root: %w", err)
	}
	return &LocalStore{root: absolute}, nil
}

func (s *LocalStore) Put(ctx context.Context, key string, data []byte, options PutOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create publication directory for %s: %w", key, err)
	}
	if options.Immutable {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s: %w", key, ErrAlreadyExists)
		}
		if err != nil {
			return fmt.Errorf("create immutable object %s: %w", key, err)
		}
		if _, err := file.Write(data); err != nil {
			file.Close()
			return fmt.Errorf("write immutable object %s: %w", key, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close immutable object %s: %w", key, err)
		}
		return nil
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pointer-*")
	if err != nil {
		return fmt.Errorf("create temporary object %s: %w", key, err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary object %s: %w", key, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary object %s: %w", key, err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace object %s: %w", key, err)
	}
	return nil
}

func (s *LocalStore) path(key string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(key))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid publication key %q", key)
	}
	return filepath.Join(s.root, cleaned), nil
}
