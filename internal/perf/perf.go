package perf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	s3crypto "s3s5/internal/crypto"
	"s3s5/internal/metrics"
	"s3s5/internal/objectstore"
	"s3s5/internal/objectstore/delay"
	"s3s5/internal/objectstore/memory"
	"s3s5/internal/policy"
	"s3s5/internal/socks5"
	"s3s5/internal/tunnel"
)

const (
	ProfileMemory      = "memory"
	ProfileSimulatedS3 = "simulated-s3"
	ProfileRealS3      = "real-s3"
)

type StoreFactory func(ctx context.Context, scenario string) (store objectstore.ObjectStore, provider string, cleanup func(context.Context) error, err error)

type Config struct {
	Profile               string
	Provider              string
	ChunkSize             int
	FlushDelay            time.Duration
	PollMin               time.Duration
	PollMax               time.Duration
	ActivePollDuration    time.Duration
	WindowChunks          uint64
	CloseCheckAfterMisses int
	IdleTimeout           time.Duration
	ShortConnections      int
	IdleSessions          int
	IdleDuration          time.Duration
	ChattyDuration        time.Duration
	ChattyInterval        time.Duration
	PrefixBase            string
	Delay                 delay.DelayProfile
	ScenarioNames         []string
	GitCommit             string
	DirtyWorktree         bool
	StoreFactory          StoreFactory
}

type RunResult struct {
	SchemaVersion int              `json:"schema_version"`
	Timestamp     time.Time        `json:"timestamp"`
	GitCommit     string           `json:"git_commit"`
	DirtyWorktree bool             `json:"dirty_worktree"`
	GoVersion     string           `json:"go_version"`
	OS            string           `json:"os"`
	Arch          string           `json:"arch"`
	Profile       string           `json:"profile"`
	Provider      string           `json:"provider"`
	Config        ProtocolConfig   `json:"config"`
	Scenarios     []ScenarioResult `json:"scenarios"`
}

type ProtocolConfig struct {
	ChunkSize             int    `json:"chunk_size"`
	FlushDelay            string `json:"flush_delay"`
	PollMin               string `json:"poll_min"`
	PollMax               string `json:"poll_max"`
	ActivePollDuration    string `json:"active_poll_duration"`
	WindowChunks          uint64 `json:"window_chunks"`
	CloseCheckAfterMisses int    `json:"close_check_after_misses"`
	IdleTimeout           string `json:"idle_timeout"`
}

type ScenarioResult struct {
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	Status             string           `json:"status"`
	Error              string           `json:"error,omitempty"`
	Parameters         map[string]any   `json:"parameters"`
	DurationMillis     int64            `json:"duration_ms"`
	Traffic            TrafficMetrics   `json:"traffic"`
	SessionMetrics     SessionMetrics   `json:"session_metrics"`
	ObjectStoreMetrics metrics.Snapshot `json:"objectstore_metrics"`
	DerivedMetrics     DerivedMetrics   `json:"derived_metrics"`
	Observations       []string         `json:"observations,omitempty"`
}

type TrafficMetrics struct {
	BytesSent       uint64 `json:"bytes_sent"`
	BytesReceived   uint64 `json:"bytes_received"`
	ExpectedBytes   uint64 `json:"expected_bytes"`
	SHA256          string `json:"sha256,omitempty"`
	Connections     int    `json:"connections"`
	Requests        int    `json:"requests"`
	IdleMillis      int64  `json:"idle_ms,omitempty"`
	RTTMillis       int64  `json:"rtt_ms,omitempty"`
	ThroughputBytes int64  `json:"throughput_bytes_per_sec,omitempty"`
}

type SessionMetrics struct {
	SessionsStarted          uint64                `json:"sessions_started"`
	SessionsOpened           uint64                `json:"sessions_opened"`
	SessionsRejected         uint64                `json:"sessions_rejected"`
	SessionsCompleted        uint64                `json:"sessions_completed"`
	SessionsFailed           uint64                `json:"sessions_failed"`
	ActiveSessions           int64                 `json:"active_sessions"`
	ChunksSent               uint64                `json:"chunks_sent"`
	ChunksReceived           uint64                `json:"chunks_received"`
	PlaintextBytesSent       uint64                `json:"plaintext_bytes_sent"`
	PlaintextBytesRecv       uint64                `json:"plaintext_bytes_received"`
	SealedBytesSent          uint64                `json:"sealed_bytes_sent"`
	SealedBytesRecv          uint64                `json:"sealed_bytes_received"`
	SocketReads              uint64                `json:"socket_reads_total"`
	AggregationFlushSize     uint64                `json:"aggregation_flush_size"`
	AggregationFlushDeadline uint64                `json:"aggregation_flush_deadline"`
	AggregationFlushEOF      uint64                `json:"aggregation_flush_eof"`
	AggregationFlushError    uint64                `json:"aggregation_flush_error"`
	TimeToOpenResult         metrics.DurationStats `json:"time_to_open_result"`
	SessionDuration          metrics.DurationStats `json:"session_duration"`
	TimeToFirstC2S           metrics.DurationStats `json:"time_to_first_c2s_object"`
	TimeToFirstS2C           metrics.DurationStats `json:"time_to_first_s2c_object"`
}

type DerivedMetrics struct {
	TotalObjectStoreOps          uint64  `json:"total_objectstore_ops"`
	S3OperationsPerSession       float64 `json:"s3_operations_per_session"`
	S3OperationsPerMiB           float64 `json:"s3_operations_per_mib"`
	GetMissesPerReceivedData     float64 `json:"get_misses_per_received_data_object"`
	HeadOperationsPerSession     float64 `json:"head_operations_per_session"`
	ListOperationsPerOpenSession float64 `json:"list_operations_per_opened_session"`
	AckGetPerDataPut             float64 `json:"ack_get_per_data_put"`
	AckPutPerReceivedData        float64 `json:"ack_put_per_received_data_object"`
	SealedPlaintextSizeRatio     float64 `json:"sealed_plaintext_size_ratio"`
}

type scenario struct {
	name        string
	description string
	run         func(context.Context, *environment, Config) (TrafficMetrics, map[string]any, error)
}

type environment struct {
	prefix    string
	provider  string
	collector *metrics.Collector
	stats     *tunnel.Stats
	socksAddr string
	stop      func()
}

func DefaultConfig(profile string) Config {
	cfg := Config{
		Profile:               profile,
		Provider:              profile,
		ChunkSize:             64 * 1024,
		FlushDelay:            10 * time.Millisecond,
		PollMin:               time.Millisecond,
		PollMax:               20 * time.Millisecond,
		ActivePollDuration:    500 * time.Millisecond,
		WindowChunks:          16,
		CloseCheckAfterMisses: 4,
		IdleTimeout:           30 * time.Second,
		ShortConnections:      20,
		IdleSessions:          20,
		IdleDuration:          10 * time.Second,
		ChattyDuration:        10 * time.Second,
		ChattyInterval:        5 * time.Millisecond,
		PrefixBase:            "perf",
	}
	if profile == ProfileSimulatedS3 {
		cfg.Delay = delay.DelayProfile{
			PutDelay:    100 * time.Millisecond,
			GetDelay:    100 * time.Millisecond,
			HeadDelay:   100 * time.Millisecond,
			ListDelay:   120 * time.Millisecond,
			DeleteDelay: 100 * time.Millisecond,
			Jitter:      10 * time.Millisecond,
			Seed:        1,
		}
		cfg.PollMin = 5 * time.Millisecond
		cfg.PollMax = 25 * time.Millisecond
	}
	return cfg
}

func Run(ctx context.Context, cfg Config) (RunResult, error) {
	cfg = normalizeConfig(cfg)
	result := RunResult{
		SchemaVersion: 1,
		Timestamp:     time.Now().UTC(),
		GitCommit:     cfg.GitCommit,
		DirtyWorktree: cfg.DirtyWorktree,
		GoVersion:     runtime.Version(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Profile:       cfg.Profile,
		Provider:      cfg.Provider,
		Config: ProtocolConfig{
			ChunkSize:             cfg.ChunkSize,
			FlushDelay:            cfg.FlushDelay.String(),
			PollMin:               cfg.PollMin.String(),
			PollMax:               cfg.PollMax.String(),
			ActivePollDuration:    cfg.ActivePollDuration.String(),
			WindowChunks:          cfg.WindowChunks,
			CloseCheckAfterMisses: cfg.CloseCheckAfterMisses,
			IdleTimeout:           cfg.IdleTimeout.String(),
		},
	}
	selected := selectScenarios(cfg.ScenarioNames)
	var failed []string
	for _, sc := range selected {
		sr := runOneScenario(ctx, cfg, sc)
		result.Scenarios = append(result.Scenarios, sr)
		if sr.Status != "passed" {
			failed = append(failed, sc.name)
		}
	}
	if len(failed) > 0 {
		return result, fmt.Errorf("performance scenarios failed: %v", failed)
	}
	return result, nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.Profile == "" {
		cfg.Profile = ProfileMemory
	}
	if cfg.Provider == "" {
		cfg.Provider = cfg.Profile
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 64 * 1024
	}
	if cfg.FlushDelay < 0 {
		cfg.FlushDelay = 0
	}
	if cfg.PollMin <= 0 {
		cfg.PollMin = time.Millisecond
	}
	if cfg.PollMax <= 0 {
		cfg.PollMax = 20 * time.Millisecond
	}
	if cfg.PollMax < cfg.PollMin {
		cfg.PollMax = cfg.PollMin
	}
	if cfg.WindowChunks == 0 {
		cfg.WindowChunks = 16
	}
	if cfg.ActivePollDuration < 0 {
		cfg.ActivePollDuration = 0
	}
	if cfg.CloseCheckAfterMisses <= 0 {
		cfg.CloseCheckAfterMisses = 4
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 30 * time.Second
	}
	if cfg.ShortConnections <= 0 {
		cfg.ShortConnections = 20
	}
	if cfg.IdleSessions <= 0 {
		cfg.IdleSessions = 20
	}
	if cfg.IdleDuration <= 0 {
		cfg.IdleDuration = 10 * time.Second
	}
	if cfg.ChattyDuration <= 0 {
		cfg.ChattyDuration = 10 * time.Second
	}
	if cfg.ChattyInterval <= 0 {
		cfg.ChattyInterval = 5 * time.Millisecond
	}
	if cfg.PrefixBase == "" {
		cfg.PrefixBase = "perf"
	}
	return cfg
}

func runOneScenario(ctx context.Context, cfg Config, sc scenario) ScenarioResult {
	start := time.Now()
	env, err := newEnvironment(ctx, cfg, sc.name)
	if err != nil {
		return ScenarioResult{Name: sc.name, Description: sc.description, Status: "failed", Error: err.Error()}
	}
	defer env.stop()
	traffic, params, err := sc.run(ctx, env, cfg)
	if params == nil {
		params = map[string]any{}
	}
	waitForSessions(ctx, env.stats, traffic.Connections, cfg.PollMax)
	duration := time.Since(start)
	snap := env.stats.Snapshot()
	storeSnap := env.collector.Snapshot()
	status := "passed"
	errText := ""
	if err != nil {
		status = "failed"
		errText = err.Error()
	}
	return ScenarioResult{
		Name:               sc.name,
		Description:        sc.description,
		Status:             status,
		Error:              errText,
		Parameters:         params,
		DurationMillis:     duration.Milliseconds(),
		Traffic:            traffic,
		SessionMetrics:     sessionMetrics(snap),
		ObjectStoreMetrics: storeSnap,
		DerivedMetrics:     deriveMetrics(traffic, snap, storeSnap),
		Observations:       observations(storeSnap),
	}
}

func newEnvironment(parent context.Context, cfg Config, scenarioName string) (*environment, error) {
	ctx, cancel := context.WithCancel(parent)
	collector := metrics.NewCollector()
	base, provider, cleanup, err := scenarioStore(ctx, cfg, scenarioName)
	if err != nil {
		cancel()
		return nil, err
	}
	store := metrics.InstrumentedStore{Next: base, Collector: collector}
	stats := &tunnel.Stats{}
	tcfg := tunnel.Config{
		Store:                 store,
		Codec:                 mustCodec(),
		Stats:                 stats,
		Prefix:                cfg.PrefixBase + "/" + scenarioName,
		ChunkSize:             cfg.ChunkSize,
		FlushDelay:            cfg.FlushDelay,
		PollMin:               cfg.PollMin,
		PollMax:               cfg.PollMax,
		ActivePollDuration:    cfg.ActivePollDuration,
		WindowChunks:          cfg.WindowChunks,
		CloseCheckAfterMisses: cfg.CloseCheckAfterMisses,
		IdleTimeout:           cfg.IdleTimeout,
	}
	pol, err := policy.New(policy.Config{AllowPrivate: true})
	if err != nil {
		cancel()
		return nil, err
	}
	server, err := tunnel.NewServer(tcfg, pol)
	if err != nil {
		cancel()
		return nil, err
	}
	server.ConnectTimeout = time.Second
	server.MaxSessions = max(32, cfg.IdleSessions+4)
	go func() { _ = server.Run(ctx) }()
	client, err := tunnel.NewClient(tcfg)
	if err != nil {
		cancel()
		return nil, err
	}
	socksAddr, stopSOCKS, err := startSOCKS(ctx, client)
	if err != nil {
		cancel()
		return nil, err
	}
	stop := func() {
		stopSOCKS()
		cancel()
		if cleanup != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = cleanup(cleanupCtx)
			cleanupCancel()
		}
	}
	return &environment{
		prefix:    tcfg.Prefix,
		provider:  provider,
		collector: collector,
		stats:     stats,
		socksAddr: socksAddr,
		stop:      stop,
	}, nil
}

func scenarioStore(ctx context.Context, cfg Config, scenarioName string) (objectstore.ObjectStore, string, func(context.Context) error, error) {
	if cfg.StoreFactory != nil {
		return cfg.StoreFactory(ctx, scenarioName)
	}
	base := objectstore.ObjectStore(memory.New())
	if cfg.Profile == ProfileSimulatedS3 {
		base = delay.New(base, cfg.Delay)
	}
	return base, cfg.Provider, nil, nil
}

func mustCodec() s3crypto.Codec {
	codec, err := s3crypto.NewPSKCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		panic(err)
	}
	return codec
}

func sessionMetrics(s tunnel.StatsSnapshot) SessionMetrics {
	return SessionMetrics{
		SessionsStarted:          s.SessionsStarted,
		SessionsOpened:           s.SessionsOpened,
		SessionsRejected:         s.SessionsRejected,
		SessionsCompleted:        s.SessionsCompleted,
		SessionsFailed:           s.SessionsFailed,
		ActiveSessions:           s.ActiveSessions,
		ChunksSent:               s.ChunksSent,
		ChunksReceived:           s.ChunksReceived,
		PlaintextBytesSent:       s.BytesSent,
		PlaintextBytesRecv:       s.BytesReceived,
		SealedBytesSent:          s.SealedBytesSent,
		SealedBytesRecv:          s.SealedBytesReceived,
		SocketReads:              s.SocketReads,
		AggregationFlushSize:     s.AggregationFlushSize,
		AggregationFlushDeadline: s.AggregationFlushDeadline,
		AggregationFlushEOF:      s.AggregationFlushEOF,
		AggregationFlushError:    s.AggregationFlushError,
		TimeToOpenResult:         metrics.SummarizeDurations(s.TimeToOpenResult),
		SessionDuration:          metrics.SummarizeDurations(s.SessionDurations),
		TimeToFirstC2S:           metrics.SummarizeDurations(s.TimeToFirstC2SData),
		TimeToFirstS2C:           metrics.SummarizeDurations(s.TimeToFirstS2CData),
	}
}

func deriveMetrics(traffic TrafficMetrics, s tunnel.StatsSnapshot, store metrics.Snapshot) DerivedMetrics {
	totalOps := countOps(store, "", "", "")
	dataPuts := countOps(store, metrics.OperationPut, metrics.KeyDataC2S, metrics.ResultSuccess) +
		countOps(store, metrics.OperationPut, metrics.KeyDataS2C, metrics.ResultSuccess)
	ackGets := countOps(store, metrics.OperationGet, metrics.KeyAckC2S, "") +
		countOps(store, metrics.OperationGet, metrics.KeyAckS2C, "")
	ackPuts := countOps(store, metrics.OperationPut, metrics.KeyAckC2S, metrics.ResultSuccess) +
		countOps(store, metrics.OperationPut, metrics.KeyAckS2C, metrics.ResultSuccess)
	dataGetMisses := countOps(store, metrics.OperationGet, metrics.KeyDataC2S, metrics.ResultNotFound) +
		countOps(store, metrics.OperationGet, metrics.KeyDataS2C, metrics.ResultNotFound)
	headOps := countOps(store, metrics.OperationHead, "", "")
	listOps := countOps(store, metrics.OperationList, "", "")
	return DerivedMetrics{
		TotalObjectStoreOps:          totalOps,
		S3OperationsPerSession:       div(float64(totalOps), float64(maxUint64(s.SessionsStarted, 1))),
		S3OperationsPerMiB:           div(float64(totalOps), float64(traffic.BytesSent)/(1024*1024)),
		GetMissesPerReceivedData:     div(float64(dataGetMisses), float64(maxUint64(s.ChunksReceived, 1))),
		HeadOperationsPerSession:     div(float64(headOps), float64(maxUint64(s.SessionsStarted, 1))),
		ListOperationsPerOpenSession: div(float64(listOps), float64(maxUint64(s.SessionsOpened, 1))),
		AckGetPerDataPut:             div(float64(ackGets), float64(maxUint64(dataPuts, 1))),
		AckPutPerReceivedData:        div(float64(ackPuts), float64(maxUint64(s.ChunksReceived, 1))),
		SealedPlaintextSizeRatio:     div(float64(s.SealedBytesSent), float64(maxUint64(s.BytesSent, 1))),
	}
}

func countOps(s metrics.Snapshot, operation, keyClass, result string) uint64 {
	var n uint64
	for _, op := range s.Operations {
		if operation != "" && op.Operation != operation {
			continue
		}
		if keyClass != "" && op.KeyClass != keyClass {
			continue
		}
		if result != "" && op.Result != result {
			continue
		}
		n += op.Count
	}
	return n
}

func observations(s metrics.Snapshot) []string {
	total := countOps(s, "", "", "")
	getMisses := countOps(s, metrics.OperationGet, "", metrics.ResultNotFound)
	head := countOps(s, metrics.OperationHead, "", "")
	list := countOps(s, metrics.OperationList, "", "")
	out := make([]string, 0, 3)
	if total > 0 && getMisses*2 >= total {
		out = append(out, "GET not_found polling misses are at least half of observed object-store operations.")
	}
	if head > 0 {
		out = append(out, "HEAD close-marker checks are visible in the request mix.")
	}
	if list > 0 {
		out = append(out, "LIST open-session polling is part of baseline request volume.")
	}
	return out
}

func div(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func maxUint64(v, min uint64) uint64 {
	if v < min {
		return min
	}
	return v
}

func waitForSessions(ctx context.Context, stats *tunnel.Stats, want int, pollMax time.Duration) {
	if want <= 0 {
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := stats.Snapshot()
		if int(s.SessionsCompleted+s.SessionsFailed+s.SessionsRejected) >= want {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(maxDuration(pollMax, 5*time.Millisecond)):
		}
	}
}

func selectScenarios(names []string) []scenario {
	all := scenarios()
	if len(names) == 0 {
		return all
	}
	byName := make(map[string]scenario, len(all))
	for _, sc := range all {
		byName[sc.name] = sc
	}
	out := make([]scenario, 0, len(names))
	for _, name := range names {
		if sc, ok := byName[name]; ok {
			out = append(out, sc)
		}
	}
	return out
}

func ScenarioNames() []string {
	all := scenarios()
	names := make([]string, 0, len(all))
	for _, sc := range all {
		names = append(names, sc.name)
	}
	return names
}

func scenarios() []scenario {
	return []scenario{
		{name: "one-byte-echo-active", description: "Open one SOCKS connection, send one byte, and read one echoed byte without a long idle period.", run: runOneByteActive},
		{name: "one-byte-echo-after-idle", description: "Open one SOCKS connection, hold it idle for the configured duration, then send and echo one byte.", run: runOneByteAfterIdle},
		{name: "small-chatty-writes", description: "Send ordered 100-byte writes for the configured duration and verify echoed order.", run: runSmallChattyWrites},
		{name: "bulk-one-mib", description: "Send and echo one contiguous MiB, verifying SHA-256 and exact byte count.", run: runBulkOneMiB},
		{name: "short-connections", description: "Open sequential short SOCKS connections and exchange a small request/response on each.", run: runShortConnections},
		{name: "concurrent-idle-sessions", description: "Open idle SOCKS sessions for a bounded period and measure background polling.", run: runConcurrentIdleSessions},
		{name: "mixed-traffic", description: "Run one bulk stream, small request/response streams, and idle streams at the same time.", run: runMixedTraffic},
	}
}

func runOneByteActive(ctx context.Context, env *environment, cfg Config) (TrafficMetrics, map[string]any, error) {
	echoAddr, stopEcho, err := startEcho()
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer stopEcho()
	conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer conn.Close()
	payload := []byte("x")
	rttStart := time.Now()
	got, err := writeRead(conn, payload)
	rtt := time.Since(rttStart)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	if !bytes.Equal(got, payload) {
		return TrafficMetrics{}, nil, fmt.Errorf("echo mismatch")
	}
	_ = conn.Close()
	return TrafficMetrics{BytesSent: 1, BytesReceived: 1, ExpectedBytes: 1, Connections: 1, Requests: 1, RTTMillis: rtt.Milliseconds()}, nil, nil
}

func runOneByteAfterIdle(ctx context.Context, env *environment, cfg Config) (TrafficMetrics, map[string]any, error) {
	echoAddr, stopEcho, err := startEcho()
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer stopEcho()
	conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer conn.Close()
	idle := cfg.IdleDuration
	if err := sleep(ctx, idle); err != nil {
		return TrafficMetrics{}, nil, err
	}
	payload := []byte("i")
	rttStart := time.Now()
	got, err := writeRead(conn, payload)
	rtt := time.Since(rttStart)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	if !bytes.Equal(got, payload) {
		return TrafficMetrics{}, nil, fmt.Errorf("echo mismatch")
	}
	_ = conn.Close()
	return TrafficMetrics{BytesSent: 1, BytesReceived: 1, ExpectedBytes: 1, Connections: 1, Requests: 1, IdleMillis: idle.Milliseconds(), RTTMillis: rtt.Milliseconds()}, map[string]any{"idle": idle.String()}, nil
}

func runSmallChattyWrites(ctx context.Context, env *environment, cfg Config) (TrafficMetrics, map[string]any, error) {
	echoAddr, stopEcho, err := startEcho()
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer stopEcho()
	conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer conn.Close()
	const size = 100
	sent, recv, writes, err := runChattyExchanges(ctx, conn, 0, cfg.ChattyDuration, cfg.ChattyInterval, size)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	_ = conn.Close()
	traffic := TrafficMetrics{
		BytesSent:     sent,
		BytesReceived: recv,
		ExpectedBytes: sent,
		Connections:   1,
		Requests:      writes,
	}
	params := map[string]any{
		"writes":     writes,
		"write_size": size,
		"duration":   cfg.ChattyDuration.String(),
		"interval":   cfg.ChattyInterval.String(),
	}
	return traffic, params, nil
}

func runBulkOneMiB(ctx context.Context, env *environment, cfg Config) (TrafficMetrics, map[string]any, error) {
	echoAddr, stopEcho, err := startEcho()
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer stopEcho()
	conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer conn.Close()
	payload := makePattern(1 << 20)
	start := time.Now()
	got, err := writeRead(conn, payload)
	elapsed := time.Since(start)
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	if !bytes.Equal(got, payload) {
		return TrafficMetrics{}, nil, fmt.Errorf("bulk echo mismatch")
	}
	sum := sha256.Sum256(got)
	_ = conn.Close()
	return TrafficMetrics{
		BytesSent:       uint64(len(payload)),
		BytesReceived:   uint64(len(got)),
		ExpectedBytes:   uint64(len(payload)),
		SHA256:          hex.EncodeToString(sum[:]),
		Connections:     1,
		Requests:        1,
		ThroughputBytes: int64(float64(len(payload)) / elapsed.Seconds()),
	}, map[string]any{"bytes": len(payload)}, nil
}

func runShortConnections(ctx context.Context, env *environment, cfg Config) (TrafficMetrics, map[string]any, error) {
	echoAddr, stopEcho, err := startEcho()
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer stopEcho()
	var sent, recv uint64
	for i := 0; i < cfg.ShortConnections; i++ {
		conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
		if err != nil {
			return TrafficMetrics{}, nil, err
		}
		payload := []byte(fmt.Sprintf("short-%02d", i))
		got, err := writeRead(conn, payload)
		_ = conn.Close()
		if err != nil {
			return TrafficMetrics{}, nil, err
		}
		if !bytes.Equal(got, payload) {
			return TrafficMetrics{}, nil, fmt.Errorf("short connection %d echo mismatch", i)
		}
		sent += uint64(len(payload))
		recv += uint64(len(got))
	}
	return TrafficMetrics{BytesSent: sent, BytesReceived: recv, ExpectedBytes: sent, Connections: cfg.ShortConnections, Requests: cfg.ShortConnections}, map[string]any{"connections": cfg.ShortConnections}, nil
}

func runConcurrentIdleSessions(ctx context.Context, env *environment, cfg Config) (TrafficMetrics, map[string]any, error) {
	echoAddr, stopEcho, err := startEcho()
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer stopEcho()
	conns := make([]net.Conn, 0, cfg.IdleSessions)
	for i := 0; i < cfg.IdleSessions; i++ {
		conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
		if err != nil {
			closeAll(conns)
			return TrafficMetrics{}, nil, err
		}
		conns = append(conns, conn)
	}
	if err := sleep(ctx, cfg.IdleDuration); err != nil {
		closeAll(conns)
		return TrafficMetrics{}, nil, err
	}
	closeAll(conns)
	return TrafficMetrics{Connections: cfg.IdleSessions, IdleMillis: cfg.IdleDuration.Milliseconds()}, map[string]any{"sessions": cfg.IdleSessions, "idle": cfg.IdleDuration.String()}, nil
}

func runMixedTraffic(ctx context.Context, env *environment, cfg Config) (TrafficMetrics, map[string]any, error) {
	echoAddr, stopEcho, err := startEcho()
	if err != nil {
		return TrafficMetrics{}, nil, err
	}
	defer stopEcho()
	var mu sync.Mutex
	var traffic TrafficMetrics
	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	add := func(sent, recv uint64, conns, requests int) {
		mu.Lock()
		traffic.BytesSent += sent
		traffic.BytesReceived += recv
		traffic.ExpectedBytes += sent
		traffic.Connections += conns
		traffic.Requests += requests
		mu.Unlock()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		payload := makePattern(256 * 1024)
		got, err := writeRead(conn, payload)
		if err != nil {
			errCh <- err
			return
		}
		if !bytes.Equal(got, payload) {
			errCh <- fmt.Errorf("mixed bulk echo mismatch")
			return
		}
		add(uint64(len(payload)), uint64(len(got)), 1, 1)
	}()
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()
			sent, recv, writes, err := runChattyExchanges(ctx, conn, id+1, cfg.ChattyDuration, cfg.ChattyInterval, 100)
			if err != nil {
				errCh <- err
				return
			}
			add(sent, recv, 1, writes)
		}(i)
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := openSOCKS(ctx, env.socksAddr, echoAddr)
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()
			_ = sleep(ctx, cfg.IdleDuration)
			add(0, 0, 1, 0)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return traffic, nil, err
		}
	}
	return traffic, map[string]any{
		"bulk_bytes":      256 * 1024,
		"chatty_streams":  3,
		"chatty_duration": cfg.ChattyDuration.String(),
		"chatty_interval": cfg.ChattyInterval.String(),
		"idle_streams":    3,
		"idle_duration":   cfg.IdleDuration.String(),
	}, nil
}

func runChattyExchanges(ctx context.Context, conn net.Conn, streamID int, duration, interval time.Duration, size int) (uint64, uint64, int, error) {
	var sent, recv uint64
	writes := 0
	deadline := time.Now().Add(duration)
	for writes == 0 || time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return sent, recv, writes, ctx.Err()
		default:
		}
		payload := makeChattyPayload(streamID, writes, size)
		got, err := writeRead(conn, payload)
		if err != nil {
			return sent, recv, writes, err
		}
		if !bytes.Equal(got, payload) {
			return sent, recv, writes, fmt.Errorf("chatty write %d echo mismatch", writes)
		}
		sent += uint64(len(payload))
		recv += uint64(len(got))
		writes++
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := interval
		if remaining < wait {
			wait = remaining
		}
		if wait > 0 {
			if err := sleep(ctx, wait); err != nil {
				return sent, recv, writes, err
			}
		}
	}
	return sent, recv, writes, nil
}

func makeChattyPayload(streamID, sequence, size int) []byte {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte((streamID*31 + sequence + i) % 251)
	}
	return payload
}

func startSOCKS(ctx context.Context, client *tunnel.Client) (string, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	go func() { _ = (&socks5.Server{Handler: client.HandleSOCKS}).Serve(ctx, ln) }()
	return ln.Addr().String(), func() { _ = ln.Close() }, nil
}

func startEcho() (string, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}, nil
}

func openSOCKS(ctx context.Context, socksAddr, target string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", socksAddr)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	}
	if err := socksHandshake(conn, target); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func socksHandshake(conn net.Conn, target string) error {
	if _, err := conn.Write([]byte{socks5.Version5, 1, socks5.MethodNoAuth}); err != nil {
		return err
	}
	method := make([]byte, 2)
	if _, err := io.ReadFull(conn, method); err != nil {
		return err
	}
	host, portRaw, err := net.SplitHostPort(target)
	if err != nil {
		return err
	}
	port64, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host).To4()
	if ip == nil {
		return fmt.Errorf("benchmark target must be IPv4: %s", target)
	}
	req := []byte{socks5.Version5, socks5.CmdConnect, 0, socks5.AtypIPv4}
	req = append(req, ip...)
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port64))
	req = append(req, p[:]...)
	if _, err := conn.Write(req); err != nil {
		return err
	}
	resp := make([]byte, 10)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[1] != socks5.ReplySucceeded {
		return fmt.Errorf("SOCKS reply %d", resp[1])
	}
	return nil
}

func writeRead(conn net.Conn, payload []byte) ([]byte, error) {
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetDeadline(time.Time{})
	got := make([]byte, len(payload))
	errCh := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(conn, got)
		errCh <- err
	}()
	if _, err := conn.Write(payload); err != nil {
		return nil, err
	}
	if err := <-errCh; err != nil {
		return nil, err
	}
	return got, nil
}

func makePattern(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(i % 251)
	}
	return out
}

func closeAll(conns []net.Conn) {
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func MarshalJSONDeterministic(v any, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func DecodeResult(r io.Reader) (RunResult, error) {
	var result RunResult
	err := json.NewDecoder(r).Decode(&result)
	return result, err
}

func ValidateResult(result RunResult) error {
	if result.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema version %d", result.SchemaVersion)
	}
	if len(result.Scenarios) == 0 {
		return errors.New("result has no scenarios")
	}
	for _, sc := range result.Scenarios {
		if sc.Status != "passed" {
			return fmt.Errorf("scenario %s status %s", sc.Name, sc.Status)
		}
	}
	return nil
}

func SortScenarioResults(result *RunResult) {
	sort.Slice(result.Scenarios, func(i, j int) bool {
		return result.Scenarios[i].Name < result.Scenarios[j].Name
	})
}
