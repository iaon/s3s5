package perf

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"s3s5/internal/metrics"
)

func RenderMarkdown(w io.Writer, result RunResult) error {
	if _, err := fmt.Fprintf(w, "# s3s5 Performance Baseline\n\n"); err != nil {
		return err
	}
	fmt.Fprintf(w, "## Environment\n\n")
	fmt.Fprintf(w, "- Commit: `%s`\n", empty(result.GitCommit, "unknown"))
	fmt.Fprintf(w, "- Dirty worktree: `%t`\n", result.DirtyWorktree)
	fmt.Fprintf(w, "- Timestamp: `%s`\n", result.Timestamp.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(w, "- Go: `%s`\n", result.GoVersion)
	fmt.Fprintf(w, "- OS/arch: `%s/%s`\n", result.OS, result.Arch)
	fmt.Fprintf(w, "- Profile/provider: `%s` / `%s`\n\n", result.Profile, result.Provider)

	fmt.Fprintf(w, "## Protocol Configuration\n\n")
	fmt.Fprintf(w, "| Chunk size | Poll min | Poll max | Window chunks | Idle timeout |\n")
	fmt.Fprintf(w, "| ---: | ---: | ---: | ---: | ---: |\n")
	fmt.Fprintf(w, "| %d | %s | %s | %d | %s |\n\n", result.Config.ChunkSize, result.Config.PollMin, result.Config.PollMax, result.Config.WindowChunks, result.Config.IdleTimeout)

	fmt.Fprintf(w, "## Scenario Summary\n\n")
	fmt.Fprintf(w, "| Scenario | Status | Bytes sent | Bytes received | Duration ms | PUT | GET hit | GET miss | HEAD | LIST | DELETE | Ops/session | Ops/MiB |\n")
	fmt.Fprintf(w, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, sc := range result.Scenarios {
		fmt.Fprintf(w, "| %s | %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %.2f | %.2f |\n",
			sc.Name,
			sc.Status,
			sc.Traffic.BytesSent,
			sc.Traffic.BytesReceived,
			sc.DurationMillis,
			countOps(sc.ObjectStoreMetrics, metrics.OperationPut, "", ""),
			countOps(sc.ObjectStoreMetrics, metrics.OperationGet, "", metrics.ResultSuccess),
			countOps(sc.ObjectStoreMetrics, metrics.OperationGet, "", metrics.ResultNotFound),
			countOps(sc.ObjectStoreMetrics, metrics.OperationHead, "", ""),
			countOps(sc.ObjectStoreMetrics, metrics.OperationList, "", ""),
			countOps(sc.ObjectStoreMetrics, metrics.OperationDelete, "", ""),
			sc.DerivedMetrics.S3OperationsPerSession,
			sc.DerivedMetrics.S3OperationsPerMiB,
		)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "## Idle Scenario Normalization\n\n")
	fmt.Fprintf(w, "| Scenario | Sessions | Idle duration ms | GET/s | HEAD/s | LIST/s | Ops/session/s |\n")
	fmt.Fprintf(w, "| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, sc := range result.Scenarios {
		if sc.Name != "concurrent-idle-sessions" {
			continue
		}
		seconds := float64(sc.Traffic.IdleMillis) / 1000
		sessions := float64(max(sc.Traffic.Connections, 1))
		fmt.Fprintf(w, "| %s | %d | %d | %.2f | %.2f | %.2f | %.2f |\n",
			sc.Name,
			sc.Traffic.Connections,
			sc.Traffic.IdleMillis,
			div(float64(countOps(sc.ObjectStoreMetrics, metrics.OperationGet, "", "")), seconds),
			div(float64(countOps(sc.ObjectStoreMetrics, metrics.OperationHead, "", "")), seconds),
			div(float64(countOps(sc.ObjectStoreMetrics, metrics.OperationList, "", "")), seconds),
			div(float64(sc.DerivedMetrics.TotalObjectStoreOps), sessions*seconds),
		)
	}
	fmt.Fprintf(w, "\n")

	for _, sc := range result.Scenarios {
		fmt.Fprintf(w, "## %s\n\n", sc.Name)
		fmt.Fprintf(w, "%s\n\n", sc.Description)
		fmt.Fprintf(w, "- Status: `%s`\n", sc.Status)
		if sc.Error != "" {
			fmt.Fprintf(w, "- Error: `%s`\n", sc.Error)
		}
		fmt.Fprintf(w, "- Connections: `%d`\n", sc.Traffic.Connections)
		fmt.Fprintf(w, "- Requests: `%d`\n", sc.Traffic.Requests)
		fmt.Fprintf(w, "- Chunks sent/received: `%d/%d`\n", sc.SessionMetrics.ChunksSent, sc.SessionMetrics.ChunksReceived)
		fmt.Fprintf(w, "- Plaintext bytes sent/received: `%d/%d`\n", sc.SessionMetrics.PlaintextBytesSent, sc.SessionMetrics.PlaintextBytesRecv)
		fmt.Fprintf(w, "- Sealed/plaintext ratio: `%.3f`\n", sc.DerivedMetrics.SealedPlaintextSizeRatio)
		fmt.Fprintf(w, "- Time to open result p50/p95/p99 ms: `%.3f/%.3f/%.3f`\n\n", sc.SessionMetrics.TimeToOpenResult.P50Millis, sc.SessionMetrics.TimeToOpenResult.P95Millis, sc.SessionMetrics.TimeToOpenResult.P99Millis)
		renderOperationTable(w, sc.ObjectStoreMetrics)
		if len(sc.Observations) > 0 {
			fmt.Fprintf(w, "\nObservations:\n\n")
			for _, obs := range sc.Observations {
				fmt.Fprintf(w, "- %s\n", obs)
			}
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "## Operations By Key Class\n\n")
	renderGlobalKeyClassTable(w, result)

	fmt.Fprintf(w, "\n## Baseline Before v1 Optimizations\n\n")
	fmt.Fprintf(w, "This report records protocol v1 behavior before optimization work. It does not change key layout, ACK behavior, close markers, chunking, polling, or Android compatibility.\n")
	return nil
}

func renderOperationTable(w io.Writer, s metrics.Snapshot) {
	fmt.Fprintf(w, "| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |\n")
	fmt.Fprintf(w, "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, op := range s.Operations {
		fmt.Fprintf(w, "| %s | %s | %s | %d | %.3f | %.3f | %.3f | %.3f | %d | %d |\n",
			op.Operation,
			op.KeyClass,
			op.Result,
			op.Count,
			op.Duration.P50Millis,
			op.Duration.P95Millis,
			op.Duration.P99Millis,
			op.Duration.MaxMillis,
			op.RequestBytes,
			op.ResponseBytes,
		)
	}
}

func renderGlobalKeyClassTable(w io.Writer, result RunResult) {
	type key struct {
		Class     string
		Operation string
	}
	counts := make(map[key]uint64)
	for _, sc := range result.Scenarios {
		for _, op := range sc.ObjectStoreMetrics.Operations {
			counts[key{Class: op.KeyClass, Operation: op.Operation}] += op.Count
		}
	}
	keys := make([]key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Class != keys[j].Class {
			return keys[i].Class < keys[j].Class
		}
		return keys[i].Operation < keys[j].Operation
	})
	fmt.Fprintf(w, "| Key class | Operation | Count |\n")
	fmt.Fprintf(w, "| --- | --- | ---: |\n")
	for _, k := range keys {
		fmt.Fprintf(w, "| %s | %s | %d |\n", k.Class, k.Operation, counts[k])
	}
}

func empty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
