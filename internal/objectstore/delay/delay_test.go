package delay

import (
	"context"
	"errors"
	"testing"
	"time"

	"s3s5/internal/objectstore"
	"s3s5/internal/objectstore/memory"
)

func TestDelayCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	store := New(memory.New(), DelayProfile{PutDelay: time.Hour})
	start := time.Now()
	err := store.PutObject(ctx, "k", []byte("v"), objectstore.PutOptions{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("PutObject err = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cancellation took too long: %s", elapsed)
	}
}

func TestDelayNoJitter(t *testing.T) {
	store := New(memory.New(), DelayProfile{})
	if err := store.DeleteObject(context.Background(), "missing"); err != nil {
		t.Fatal(err)
	}
}
