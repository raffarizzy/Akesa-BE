package storage

import (
	"context"
	"io"
)

type FileStorage interface {
	Upload(ctx context.Context, key string, reader io.Reader) error
	Delete(ctx context.Context, key string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}
