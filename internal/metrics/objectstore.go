package metrics

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"s3s5/internal/objectstore"
)

const (
	OperationPut    = "put"
	OperationGet    = "get"
	OperationHead   = "head"
	OperationList   = "list"
	OperationDelete = "delete"

	KeyOpen            = "open"
	KeyOpenResult      = "open-result"
	KeyDataC2S         = "data-c2s"
	KeyDataS2C         = "data-s2c"
	KeyAckC2S          = "ack-c2s"
	KeyAckS2C          = "ack-s2c"
	KeyCloseClient     = "close-client"
	KeyCloseServer     = "close-server"
	KeyHeartbeatClient = "heartbeat-client"
	KeyHeartbeatServer = "heartbeat-server"
	KeyListOpen        = "list-open"
	KeyUnknown         = "unknown"

	ResultSuccess   = "success"
	ResultNotFound  = "not_found"
	ResultCancelled = "cancelled"
	ResultTimeout   = "timeout"
	ResultError     = "error"
)

type Collector struct {
	mu        sync.Mutex
	operation map[operationKey]*operationAggregate
}

type operationKey struct {
	Operation string
	KeyClass  string
	Result    string
}

type operationAggregate struct {
	Count         uint64
	Durations     []time.Duration
	RequestBytes  uint64
	ResponseBytes uint64
}

type Snapshot struct {
	Operations []OperationMetrics `json:"operations"`
}

type OperationMetrics struct {
	Operation     string        `json:"operation"`
	KeyClass      string        `json:"key_class"`
	Result        string        `json:"result"`
	Count         uint64        `json:"count"`
	Duration      DurationStats `json:"duration"`
	RequestBytes  uint64        `json:"request_bytes"`
	ResponseBytes uint64        `json:"response_bytes"`
}

type DurationStats struct {
	Count       uint64  `json:"count"`
	TotalMillis float64 `json:"total_ms"`
	MinMillis   float64 `json:"min_ms"`
	P50Millis   float64 `json:"p50_ms"`
	P95Millis   float64 `json:"p95_ms"`
	P99Millis   float64 `json:"p99_ms"`
	MaxMillis   float64 `json:"max_ms"`
}

func NewCollector() *Collector {
	return &Collector{operation: make(map[operationKey]*operationAggregate)}
}

func (c *Collector) ObserveObjectStore(operation, keyClass, result string, duration time.Duration, requestBytes, responseBytes uint64) {
	if c == nil {
		return
	}
	k := operationKey{Operation: operation, KeyClass: keyClass, Result: result}
	c.mu.Lock()
	if c.operation == nil {
		c.operation = make(map[operationKey]*operationAggregate)
	}
	agg := c.operation[k]
	if agg == nil {
		agg = &operationAggregate{}
		c.operation[k] = agg
	}
	agg.Count++
	agg.Durations = append(agg.Durations, duration)
	agg.RequestBytes += requestBytes
	agg.ResponseBytes += responseBytes
	c.mu.Unlock()
}

func (c *Collector) Snapshot() Snapshot {
	if c == nil {
		return Snapshot{}
	}
	c.mu.Lock()
	ops := make([]OperationMetrics, 0, len(c.operation))
	for k, agg := range c.operation {
		samples := append([]time.Duration(nil), agg.Durations...)
		ops = append(ops, OperationMetrics{
			Operation:     k.Operation,
			KeyClass:      k.KeyClass,
			Result:        k.Result,
			Count:         agg.Count,
			Duration:      SummarizeDurations(samples),
			RequestBytes:  agg.RequestBytes,
			ResponseBytes: agg.ResponseBytes,
		})
	}
	c.mu.Unlock()
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Operation != ops[j].Operation {
			return ops[i].Operation < ops[j].Operation
		}
		if ops[i].KeyClass != ops[j].KeyClass {
			return ops[i].KeyClass < ops[j].KeyClass
		}
		return ops[i].Result < ops[j].Result
	})
	return Snapshot{Operations: ops}
}

func SummarizeDurations(samples []time.Duration) DurationStats {
	if len(samples) == 0 {
		return DurationStats{}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	var total time.Duration
	for _, d := range samples {
		total += d
	}
	return DurationStats{
		Count:       uint64(len(samples)),
		TotalMillis: millis(total),
		MinMillis:   millis(samples[0]),
		P50Millis:   millis(percentile(samples, 0.50)),
		P95Millis:   millis(percentile(samples, 0.95)),
		P99Millis:   millis(percentile(samples, 0.99)),
		MaxMillis:   millis(samples[len(samples)-1]),
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := int(p*float64(len(sorted)-1) + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func millis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

type InstrumentedStore struct {
	Next      objectstore.ObjectStore
	Collector *Collector
}

func (s InstrumentedStore) PutObject(ctx context.Context, key string, data []byte, opts objectstore.PutOptions) error {
	start := time.Now()
	err := s.Next.PutObject(ctx, key, data, opts)
	s.observe(OperationPut, ClassifyKey(key), err, time.Since(start), uint64(len(data)), 0)
	return err
}

func (s InstrumentedStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()
	data, err := s.Next.GetObject(ctx, key)
	var responseBytes uint64
	if err == nil {
		responseBytes = uint64(len(data))
	}
	s.observe(OperationGet, ClassifyKey(key), err, time.Since(start), 0, responseBytes)
	return data, err
}

func (s InstrumentedStore) HeadObject(ctx context.Context, key string) (objectstore.ObjectInfo, error) {
	start := time.Now()
	info, err := s.Next.HeadObject(ctx, key)
	s.observe(OperationHead, ClassifyKey(key), err, time.Since(start), 0, 0)
	return info, err
}

func (s InstrumentedStore) ListPrefix(ctx context.Context, prefix string, opts objectstore.ListOptions) ([]string, error) {
	page, err := s.ListPrefixPage(ctx, prefix, opts)
	return page.Keys, err
}

func (s InstrumentedStore) ListPrefixPage(ctx context.Context, prefix string, opts objectstore.ListOptions) (objectstore.ListPage, error) {
	start := time.Now()
	page, err := s.Next.ListPrefixPage(ctx, prefix, opts)
	var responseBytes uint64
	if err == nil {
		for _, key := range page.Keys {
			responseBytes += uint64(len(key))
		}
	}
	s.observe(OperationList, ClassifyListPrefix(prefix), err, time.Since(start), 0, responseBytes)
	return page, err
}

func (s InstrumentedStore) DeleteObject(ctx context.Context, key string) error {
	start := time.Now()
	err := s.Next.DeleteObject(ctx, key)
	s.observe(OperationDelete, ClassifyKey(key), err, time.Since(start), 0, 0)
	return err
}

func (s InstrumentedStore) observe(operation, keyClass string, err error, d time.Duration, requestBytes, responseBytes uint64) {
	if s.Collector != nil {
		s.Collector.ObserveObjectStore(operation, keyClass, ClassifyResult(err), d, requestBytes, responseBytes)
	}
}

func ClassifyResult(err error) string {
	switch {
	case err == nil:
		return ResultSuccess
	case objectstore.IsNotFound(err):
		return ResultNotFound
	case errors.Is(err, context.Canceled):
		return ResultCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return ResultTimeout
	default:
		return ResultError
	}
}

func ClassifyListPrefix(prefix string) string {
	if ClassifyKey(strings.TrimSuffix(prefix, "/")+"/placeholder.json") == KeyOpen {
		return KeyListOpen
	}
	return KeyUnknown
}

func ClassifyKey(key string) string {
	parts := splitKey(key)
	v1 := -1
	for i, part := range parts {
		if part == "v1" {
			v1 = i
			break
		}
	}
	if v1 < 0 || v1+1 >= len(parts) {
		return KeyUnknown
	}
	parts = parts[v1+1:]
	switch parts[0] {
	case "open":
		if len(parts) == 2 {
			return KeyOpen
		}
	case "open-result":
		if len(parts) == 2 {
			return KeyOpenResult
		}
	case "data":
		if len(parts) >= 4 {
			switch parts[1] {
			case "c2s":
				return KeyDataC2S
			case "s2c":
				return KeyDataS2C
			}
		}
	case "ack":
		if len(parts) >= 3 {
			switch parts[1] {
			case "c2s":
				return KeyAckC2S
			case "s2c":
				return KeyAckS2C
			}
		}
	case "close":
		if len(parts) >= 3 {
			switch parts[1] {
			case "client":
				return KeyCloseClient
			case "server":
				return KeyCloseServer
			}
		}
	case "heartbeat":
		if len(parts) >= 3 {
			switch parts[1] {
			case "client":
				return KeyHeartbeatClient
			case "server":
				return KeyHeartbeatServer
			}
		}
	}
	return KeyUnknown
}

func splitKey(key string) []string {
	clean := path.Clean(strings.Trim(key, "/"))
	if clean == "." || clean == "" {
		return nil
	}
	return strings.Split(clean, "/")
}
