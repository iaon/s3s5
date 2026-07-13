package perf

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMemoryPerformanceSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg := DefaultConfig(ProfileMemory)
	cfg.ScenarioNames = []string{"one-byte-echo-active", "small-chatty-writes", "bulk-one-mib", "short-connections", "concurrent-idle-sessions", "mixed-traffic"}
	cfg.ShortConnections = 3
	cfg.IdleSessions = 3
	cfg.IdleDuration = 20 * time.Millisecond
	cfg.ChattyDuration = 20 * time.Millisecond
	cfg.ChattyInterval = time.Millisecond
	cfg.PollMax = 5 * time.Millisecond
	result, err := Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Scenarios) != len(cfg.ScenarioNames) {
		t.Fatalf("scenario count = %d", len(result.Scenarios))
	}
	for _, sc := range result.Scenarios {
		if sc.Status != "passed" {
			t.Fatalf("%s status = %s error=%s", sc.Name, sc.Status, sc.Error)
		}
		if sc.DerivedMetrics.TotalObjectStoreOps == 0 {
			t.Fatalf("%s has no object-store ops", sc.Name)
		}
	}
}

func TestReportFromFixture(t *testing.T) {
	result := RunResult{
		SchemaVersion: 1,
		Profile:       ProfileMemory,
		Provider:      ProfileMemory,
		Config: ProtocolConfig{
			ChunkSize:    1,
			PollMin:      "1ms",
			PollMax:      "1ms",
			WindowChunks: 1,
			IdleTimeout:  "1s",
		},
		Scenarios: []ScenarioResult{{
			Name:        "fixture",
			Description: "fixture scenario",
			Status:      "passed",
			Traffic:     TrafficMetrics{BytesSent: 1, BytesReceived: 1, Connections: 1},
			DerivedMetrics: DerivedMetrics{
				TotalObjectStoreOps:    1,
				S3OperationsPerSession: 1,
			},
		}},
	}
	var buf bytes.Buffer
	if err := RenderMarkdown(&buf, result); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("fixture")) {
		t.Fatalf("report does not contain fixture scenario:\n%s", buf.String())
	}
}

func TestJSONDeterministicShape(t *testing.T) {
	result := RunResult{SchemaVersion: 1, Profile: ProfileMemory, Provider: ProfileMemory}
	var a, b bytes.Buffer
	if err := MarshalJSONDeterministic(result, &a); err != nil {
		t.Fatal(err)
	}
	if err := MarshalJSONDeterministic(result, &b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatalf("json differs:\n%s\n%s", a.String(), b.String())
	}
	var decoded RunResult
	if err := json.Unmarshal(a.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
}
