package model

import (
	"context"
	"io"
)

type Storage interface {
	Upload(ctx context.Context, key string, reader io.Reader) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
