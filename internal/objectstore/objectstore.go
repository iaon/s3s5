package objectstore

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("object not found")

type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

type ListOptions struct {
	MaxKeys int
}

type ObjectInfo struct {
	Key      string
	Size     int64
	Metadata map[string]string
}

type ObjectStore interface {
	PutObject(ctx context.Context, key string, data []byte, opts PutOptions) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	HeadObject(ctx context.Context, key string) (ObjectInfo, error)
	ListPrefix(ctx context.Context, prefix string, opts ListOptions) ([]string, error)
	DeleteObject(ctx context.Context, key string) error
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func DeletePrefix(ctx context.Context, store ObjectStore, prefix string) (int, error) {
	deleted := 0
	for {
		keys, err := store.ListPrefix(ctx, prefix, ListOptions{MaxKeys: 1000})
		if err != nil {
			return deleted, err
		}
		if len(keys) == 0 {
			return deleted, nil
		}
		for _, key := range keys {
			if err := store.DeleteObject(ctx, key); err != nil && !IsNotFound(err) {
				return deleted, err
			}
			deleted++
		}
	}
}
