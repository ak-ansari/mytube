package storage

import (
	"context"
	"io"
)

type ObjectStore interface {
	Put(ctx context.Context, key string, file io.Reader, size int64) (string, error)
	Get(ctx context.Context, key string) (io.Reader, int64, error)
	Delete(ctx context.Context, key string) error
	GetUrl(ctx context.Context, key string) (string, error)
}
