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
	MaxKeys           int
	ContinuationToken string
}

type ObjectInfo struct {
	Key      string
	Size     int64
	Metadata map[string]string
}

type ListPage struct {
	Keys                  []string
	IsTruncated           bool
	NextContinuationToken string
}

type ObjectStore interface {
	PutObject(ctx context.Context, key string, data []byte, opts PutOptions) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	HeadObject(ctx context.Context, key string) (ObjectInfo, error)
	ListPrefix(ctx context.Context, prefix string, opts ListOptions) ([]string, error)
	ListPrefixPage(ctx context.Context, prefix string, opts ListOptions) (ListPage, error)
	DeleteObject(ctx context.Context, key string) error
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func DeletePrefix(ctx context.Context, store ObjectStore, prefix string) (int, error) {
	deleted := 0
	for {
		page, err := store.ListPrefixPage(ctx, prefix, ListOptions{MaxKeys: 1000})
		if err != nil {
			return deleted, err
		}
		if len(page.Keys) == 0 {
			return deleted, nil
		}
		for _, key := range page.Keys {
			if err := store.DeleteObject(ctx, key); err != nil && !IsNotFound(err) {
				return deleted, err
			}
			deleted++
		}
	}
}
