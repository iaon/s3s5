package memory

import (
	"context"
	"fmt"
	"testing"

	"s3s5/internal/objectstore"
)

func TestListPrefixPageContinuation(t *testing.T) {
	ctx := context.Background()
	store := New()
	for i := 0; i < 5; i++ {
		if err := store.PutObject(ctx, fmt.Sprintf("p/%02d", i), []byte("x"), objectstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.ListPrefixPage(ctx, "p/", objectstore.ListOptions{MaxKeys: 2})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(first.Keys) != "[p/00 p/01]" || !first.IsTruncated || first.NextContinuationToken != "p/01" {
		t.Fatalf("first page = %+v", first)
	}
	second, err := store.ListPrefixPage(ctx, "p/", objectstore.ListOptions{MaxKeys: 2, ContinuationToken: first.NextContinuationToken})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(second.Keys) != "[p/02 p/03]" || !second.IsTruncated || second.NextContinuationToken != "p/03" {
		t.Fatalf("second page = %+v", second)
	}
	third, err := store.ListPrefixPage(ctx, "p/", objectstore.ListOptions{MaxKeys: 2, ContinuationToken: second.NextContinuationToken})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(third.Keys) != "[p/04]" || third.IsTruncated || third.NextContinuationToken != "" {
		t.Fatalf("third page = %+v", third)
	}
}
