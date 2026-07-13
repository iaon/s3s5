package delay

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"s3s5/internal/objectstore"
)

type DelayProfile struct {
	PutDelay    time.Duration
	GetDelay    time.Duration
	HeadDelay   time.Duration
	ListDelay   time.Duration
	DeleteDelay time.Duration
	Jitter      time.Duration
	Seed        int64
}

type Store struct {
	Next    objectstore.ObjectStore
	Profile DelayProfile

	mu  sync.Mutex
	rng *rand.Rand
}

func New(next objectstore.ObjectStore, profile DelayProfile) *Store {
	if profile.Seed == 0 {
		profile.Seed = 1
	}
	return &Store{
		Next:    next,
		Profile: profile,
		rng:     rand.New(rand.NewSource(profile.Seed)),
	}
}

func (s *Store) PutObject(ctx context.Context, key string, data []byte, opts objectstore.PutOptions) error {
	if err := s.wait(ctx, s.Profile.PutDelay); err != nil {
		return err
	}
	return s.Next.PutObject(ctx, key, data, opts)
}

func (s *Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	if err := s.wait(ctx, s.Profile.GetDelay); err != nil {
		return nil, err
	}
	return s.Next.GetObject(ctx, key)
}

func (s *Store) HeadObject(ctx context.Context, key string) (objectstore.ObjectInfo, error) {
	if err := s.wait(ctx, s.Profile.HeadDelay); err != nil {
		return objectstore.ObjectInfo{}, err
	}
	return s.Next.HeadObject(ctx, key)
}

func (s *Store) ListPrefix(ctx context.Context, prefix string, opts objectstore.ListOptions) ([]string, error) {
	if err := s.wait(ctx, s.Profile.ListDelay); err != nil {
		return nil, err
	}
	return s.Next.ListPrefix(ctx, prefix, opts)
}

func (s *Store) DeleteObject(ctx context.Context, key string) error {
	if err := s.wait(ctx, s.Profile.DeleteDelay); err != nil {
		return err
	}
	return s.Next.DeleteObject(ctx, key)
}

func (s *Store) wait(ctx context.Context, base time.Duration) error {
	d := base + s.jitter()
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Store) jitter() time.Duration {
	if s.Profile.Jitter <= 0 {
		return 0
	}
	s.mu.Lock()
	n := s.rng.Int63n(int64(s.Profile.Jitter) + 1)
	s.mu.Unlock()
	return time.Duration(n)
}
