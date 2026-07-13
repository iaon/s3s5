package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"s3s5/internal/objectstore"
	"s3s5/internal/objectstore/memory"
)

func TestClassifyKeyDoesNotExposeSessionID(t *testing.T) {
	session := "0123456789abcdef0123456789abcdef"
	tests := map[string]string{
		"prefix/v1/open/" + session + ".json":                         KeyOpen,
		"prefix/v1/open-result/" + session + ".json":                  KeyOpenResult,
		"prefix/v1/data/c2s/" + session + "/00000000000000000001.bin": KeyDataC2S,
		"prefix/v1/data/s2c/" + session + "/00000000000000000001.bin": KeyDataS2C,
		"prefix/v1/ack/c2s/" + session + ".json":                      KeyAckC2S,
		"prefix/v1/ack/s2c/" + session + ".json":                      KeyAckS2C,
		"prefix/v1/close/client/" + session + ".json":                 KeyCloseClient,
		"prefix/v1/close/server/" + session + ".json":                 KeyCloseServer,
		"prefix/v1/heartbeat/client/" + session + ".json":             KeyHeartbeatClient,
		"prefix/v1/heartbeat/server/" + session + ".json":             KeyHeartbeatServer,
	}
	for key, want := range tests {
		if got := ClassifyKey(key); got != want {
			t.Fatalf("ClassifyKey(%q) = %q, want %q", key, got, want)
		}
		if got := ClassifyKey(key); got == session {
			t.Fatalf("session ID leaked as key class")
		}
	}
	if got := ClassifyListPrefix("prefix/v1/open/"); got != KeyListOpen {
		t.Fatalf("ClassifyListPrefix = %q, want %q", got, KeyListOpen)
	}
}

func TestClassifyResult(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ResultSuccess},
		{objectstore.ErrNotFound, ResultNotFound},
		{context.Canceled, ResultCancelled},
		{context.DeadlineExceeded, ResultTimeout},
		{errors.New("boom"), ResultError},
	}
	for _, tt := range tests {
		if got := ClassifyResult(tt.err); got != tt.want {
			t.Fatalf("ClassifyResult(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestInstrumentedStoreBytesAndResults(t *testing.T) {
	ctx := context.Background()
	collector := NewCollector()
	store := InstrumentedStore{Next: memory.New(), Collector: collector}
	key := "prefix/v1/data/c2s/0123456789abcdef0123456789abcdef/00000000000000000000.bin"
	if err := store.PutObject(ctx, key, []byte("abc"), objectstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetObject(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetObject(ctx, "prefix/v1/data/c2s/missing/00000000000000000000.bin"); !objectstore.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
	snap := collector.Snapshot()
	var put, get, miss OperationMetrics
	for _, op := range snap.Operations {
		switch {
		case op.Operation == OperationPut && op.Result == ResultSuccess:
			put = op
		case op.Operation == OperationGet && op.Result == ResultSuccess:
			get = op
		case op.Operation == OperationGet && op.Result == ResultNotFound:
			miss = op
		}
	}
	if put.Count != 1 || put.RequestBytes != 3 {
		t.Fatalf("put metrics = %+v", put)
	}
	if get.Count != 1 || get.ResponseBytes != 3 {
		t.Fatalf("get metrics = %+v", get)
	}
	if miss.Count != 1 || miss.KeyClass != KeyDataC2S {
		t.Fatalf("miss metrics = %+v", miss)
	}
}

func TestSummarizeDurationsPercentiles(t *testing.T) {
	got := SummarizeDurations([]time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	})
	if got.P50Millis != 3 || got.P95Millis != 5 || got.P99Millis != 5 || got.MaxMillis != 5 {
		t.Fatalf("duration stats = %+v", got)
	}
}
