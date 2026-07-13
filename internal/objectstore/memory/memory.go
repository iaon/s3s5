package memory

import (
	"context"
	"sort"
	"strings"
	"sync"

	"s3s5/internal/objectstore"
)

type Store struct {
	mu      sync.RWMutex
	objects map[string]object
}

type object struct {
	data     []byte
	metadata map[string]string
}

func New() *Store {
	return &Store{objects: make(map[string]object)}
}

func (s *Store) PutObject(ctx context.Context, key string, data []byte, opts objectstore.PutOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cp := append([]byte(nil), data...)
	meta := copyMetadata(opts.Metadata)
	s.mu.Lock()
	s.objects[key] = object{data: cp, metadata: meta}
	s.mu.Unlock()
	return nil
}

func (s *Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	obj, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return nil, objectstore.ErrNotFound
	}
	return append([]byte(nil), obj.data...), nil
}

func (s *Store) HeadObject(ctx context.Context, key string) (objectstore.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return objectstore.ObjectInfo{}, err
	}
	s.mu.RLock()
	obj, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return objectstore.ObjectInfo{Key: key, Size: int64(len(obj.data)), Metadata: copyMetadata(obj.metadata)}, nil
}

func (s *Store) ListPrefix(ctx context.Context, prefix string, opts objectstore.ListOptions) ([]string, error) {
	page, err := s.ListPrefixPage(ctx, prefix, opts)
	return page.Keys, err
}

func (s *Store) ListPrefixPage(ctx context.Context, prefix string, opts objectstore.ListOptions) (objectstore.ListPage, error) {
	if err := ctx.Err(); err != nil {
		return objectstore.ListPage{}, err
	}
	s.mu.RLock()
	keys := make([]string, 0)
	for key := range s.objects {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	s.mu.RUnlock()
	sort.Strings(keys)
	start := 0
	if opts.ContinuationToken != "" {
		start = sort.SearchStrings(keys, opts.ContinuationToken)
		if start < len(keys) && keys[start] == opts.ContinuationToken {
			start++
		}
	}
	if start > len(keys) {
		start = len(keys)
	}
	keys = keys[start:]
	page := objectstore.ListPage{Keys: append([]string(nil), keys...)}
	if opts.MaxKeys > 0 && len(keys) > opts.MaxKeys {
		page.Keys = append([]string(nil), keys[:opts.MaxKeys]...)
		page.IsTruncated = true
		page.NextContinuationToken = page.Keys[len(page.Keys)-1]
	}
	return page, nil
}

func (s *Store) DeleteObject(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.objects, key)
	s.mu.Unlock()
	return nil
}

func copyMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
