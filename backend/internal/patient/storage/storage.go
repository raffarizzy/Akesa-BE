package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) (*LocalStorage, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	return &LocalStorage{
		root: root,
	}, nil
}

func (s *LocalStorage) Upload(
	ctx context.Context,
	key string,
	reader io.Reader,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	cleanKey := filepath.Clean(filepath.FromSlash(key))

	if cleanKey == "." ||
		strings.HasPrefix(cleanKey, ".."+string(os.PathSeparator)) ||
		cleanKey == ".." {
		return fmt.Errorf("invalid storage key")
	}

	path := filepath.Join(s.root, cleanKey)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0600,
	)
	if err != nil {
		return fmt.Errorf("create storage file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("write storage file: %w", err)
	}

	return nil
}

func (s *LocalStorage) Delete(
	ctx context.Context,
	key string,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	cleanKey := filepath.Clean(filepath.FromSlash(key))

	if cleanKey == "." ||
		strings.HasPrefix(cleanKey, ".."+string(os.PathSeparator)) ||
		cleanKey == ".." {
		return fmt.Errorf("invalid storage key")
	}

	path := filepath.Join(s.root, cleanKey)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete storage file: %w", err)
	}

	return nil
}

func (s *LocalStorage) Open(
	ctx context.Context,
	key string,
) (io.ReadCloser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cleanKey := filepath.Clean(filepath.FromSlash(key))

	if cleanKey == "." ||
		strings.HasPrefix(cleanKey, ".."+string(os.PathSeparator)) ||
		cleanKey == ".." {
		return nil, fmt.Errorf("invalid storage key")
	}

	path := filepath.Join(s.root, cleanKey)

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open storage file: %w", err)
	}

	return file, nil
}
