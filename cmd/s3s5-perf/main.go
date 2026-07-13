package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"s3s5/internal/objectstore"
	s3store "s3s5/internal/objectstore/s3"
	"s3s5/internal/perf"
	"s3s5/internal/protocol"
	"s3s5/internal/version"
)

type scenarioFlags []string

func (s *scenarioFlags) String() string { return strings.Join(*s, ",") }
func (s *scenarioFlags) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	case "report":
		report(os.Args[2:])
	case "validate":
		validate(os.Args[2:])
	case "baseline":
		baseline(os.Args[2:])
	case "scenarios":
		for _, name := range perf.ScenarioNames() {
			fmt.Println(name)
		}
	case "version":
		fmt.Fprintf(os.Stdout, "s3s5-perf %s\n", version.String())
	default:
		usage()
		os.Exit(2)
	}
}

func run(args []string) {
	var provider, bucket, prefix, region, endpoint string
	var forcePathStyle bool
	var scenarios scenarioFlags
	var out string
	var timeout time.Duration
	var optInReal bool
	cfg := perf.DefaultConfig(perf.ProfileMemory)
	fs := flag.NewFlagSet("s3s5-perf run", flag.ExitOnError)
	fs.StringVar(&provider, "provider", getenv("S3S5_PROVIDER", ""), "real-s3 provider preset: aws, yandex, minio, custom")
	fs.StringVar(&bucket, "bucket", getenv("S3S5_BUCKET", ""), "real-s3 bucket name")
	fs.StringVar(&prefix, "prefix", getenv("S3S5_PREFIX", "s3s5"), "real-s3 benchmark key prefix")
	fs.StringVar(&region, "region", firstEnv("S3S5_REGION", "AWS_REGION", "us-east-1"), "real-s3 region")
	fs.StringVar(&endpoint, "endpoint", getenv("S3S5_ENDPOINT", ""), "real-s3 endpoint URL")
	fs.BoolVar(&forcePathStyle, "force-path-style", getenvBool("S3S5_FORCE_PATH_STYLE", false), "use path-style S3 URLs")
	fs.StringVar(&cfg.Profile, "profile", perf.ProfileMemory, "profile: memory, simulated-s3, real-s3")
	fs.StringVar(&out, "out", "benchmarks/results/local/perf.json", "output JSON path")
	fs.Var(&scenarios, "scenario", "scenario to run; repeatable; default runs all scenarios")
	fs.IntVar(&cfg.ChunkSize, "chunk-size", cfg.ChunkSize, "tunnel chunk size")
	fs.DurationVar(&cfg.PollMin, "poll-min", cfg.PollMin, "minimum polling delay")
	fs.DurationVar(&cfg.PollMax, "poll-max", cfg.PollMax, "maximum polling delay")
	fs.Uint64Var(&cfg.WindowChunks, "window-chunks", cfg.WindowChunks, "window chunks")
	fs.DurationVar(&cfg.IdleTimeout, "idle-timeout", cfg.IdleTimeout, "idle timeout")
	fs.IntVar(&cfg.ShortConnections, "short-connections", cfg.ShortConnections, "short-connections scenario count")
	fs.IntVar(&cfg.IdleSessions, "idle-sessions", cfg.IdleSessions, "concurrent idle sessions")
	fs.DurationVar(&cfg.IdleDuration, "idle-duration", cfg.IdleDuration, "idle scenario duration")
	fs.DurationVar(&cfg.Delay.PutDelay, "put-delay", 0, "simulated profile PUT delay")
	fs.DurationVar(&cfg.Delay.GetDelay, "get-delay", 0, "simulated profile GET delay")
	fs.DurationVar(&cfg.Delay.HeadDelay, "head-delay", 0, "simulated profile HEAD delay")
	fs.DurationVar(&cfg.Delay.ListDelay, "list-delay", 0, "simulated profile LIST delay")
	fs.DurationVar(&cfg.Delay.DeleteDelay, "delete-delay", 0, "simulated profile DELETE delay")
	fs.DurationVar(&cfg.Delay.Jitter, "jitter", 0, "simulated profile jitter")
	fs.Int64Var(&cfg.Delay.Seed, "jitter-seed", 1, "simulated jitter seed")
	fs.DurationVar(&timeout, "timeout", 2*time.Minute, "overall run timeout")
	fs.BoolVar(&optInReal, "real-s3-opt-in", false, "allow real-s3 profile to use configured bucket")
	_ = fs.Parse(args)

	seen := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})
	defaulted := perf.DefaultConfig(cfg.Profile)
	mergeProfileDefaults(&cfg, defaulted, seen)
	cfg.ScenarioNames = scenarios
	cfg.GitCommit = gitCommit()
	cfg.DirtyWorktree = dirtyWorktree()
	if cfg.Profile == perf.ProfileRealS3 {
		if !optInReal && os.Getenv("S3S5_PERF_REAL_S3") != "1" {
			fatal(fmt.Errorf("real-s3 profile requires --real-s3-opt-in or S3S5_PERF_REAL_S3=1"))
		}
		if bucket == "" {
			fatal(fmt.Errorf("--bucket or S3S5_BUCKET is required for real-s3 profile"))
		}
		rootPrefix := protocol.NormalizePrefix(prefix) + "/bench-" + randomHex(6)
		cfg.PrefixBase = rootPrefix
		cfg.Provider = provider
		cfg.StoreFactory = func(ctx context.Context, scenario string) (objectstore.ObjectStore, string, func(context.Context) error, error) {
			store, err := s3store.New(s3store.Config{
				Provider:       provider,
				Bucket:         bucket,
				Region:         region,
				Endpoint:       endpoint,
				ForcePathStyle: forcePathStyle,
			})
			if err != nil {
				return nil, "", nil, err
			}
			prefix := rootPrefix + "/" + scenario + "/"
			cleanup := func(cleanupCtx context.Context) error {
				_, err := objectstore.DeletePrefix(cleanupCtx, store, prefix)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cleanup failed; benchmark prefix left under %s\n", prefix)
				}
				return err
			}
			return store, provider, cleanup, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result, err := perf.Run(ctx, cfg)
	if writeErr := writeJSON(out, result); writeErr != nil {
		fatal(writeErr)
	}
	if err != nil {
		fatal(err)
	}
}

func report(args []string) {
	var in, out string
	fs := flag.NewFlagSet("s3s5-perf report", flag.ExitOnError)
	fs.StringVar(&in, "in", "benchmarks/results/baseline-v1-memory.json", "input JSON path")
	fs.StringVar(&out, "out", "benchmarks/reports/baseline-v1.md", "output Markdown path")
	_ = fs.Parse(args)
	result, err := readResult(in)
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		fatal(err)
	}
	f, err := os.Create(out)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	if err := perf.RenderMarkdown(f, result); err != nil {
		fatal(err)
	}
}

func validate(args []string) {
	var in string
	fs := flag.NewFlagSet("s3s5-perf validate", flag.ExitOnError)
	fs.StringVar(&in, "in", "benchmarks/results/baseline-v1-memory.json", "input JSON path")
	_ = fs.Parse(args)
	result, err := readResult(in)
	if err != nil {
		fatal(err)
	}
	if err := perf.ValidateResult(result); err != nil {
		fatal(err)
	}
}

func baseline(args []string) {
	fs := flag.NewFlagSet("s3s5-perf baseline", flag.ExitOnError)
	var simulated bool
	fs.BoolVar(&simulated, "simulated", true, "also update simulated-s3 baseline")
	_ = fs.Parse(args)
	run([]string{"-profile", perf.ProfileMemory, "-out", "benchmarks/results/baseline-v1-memory.json"})
	report([]string{"-in", "benchmarks/results/baseline-v1-memory.json", "-out", "benchmarks/reports/baseline-v1.md"})
	if simulated {
		run([]string{"-profile", perf.ProfileSimulatedS3, "-out", "benchmarks/results/baseline-v1-simulated-s3.json", "-short-connections", "3", "-idle-sessions", "3", "-idle-duration", "50ms", "-put-delay", "10ms", "-get-delay", "10ms", "-head-delay", "10ms", "-list-delay", "12ms", "-delete-delay", "10ms", "-jitter", "1ms"})
	}
}

func mergeProfileDefaults(cfg *perf.Config, defaults perf.Config, seen map[string]bool) {
	if cfg.Provider == "" || cfg.Provider == perf.ProfileMemory {
		cfg.Provider = defaults.Provider
	}
	if !seen["put-delay"] {
		cfg.Delay.PutDelay = defaults.Delay.PutDelay
	}
	if !seen["get-delay"] {
		cfg.Delay.GetDelay = defaults.Delay.GetDelay
	}
	if !seen["head-delay"] {
		cfg.Delay.HeadDelay = defaults.Delay.HeadDelay
	}
	if !seen["list-delay"] {
		cfg.Delay.ListDelay = defaults.Delay.ListDelay
	}
	if !seen["delete-delay"] {
		cfg.Delay.DeleteDelay = defaults.Delay.DeleteDelay
	}
	if !seen["jitter"] {
		cfg.Delay.Jitter = defaults.Delay.Jitter
	}
	if cfg.PrefixBase == "" {
		cfg.PrefixBase = defaults.PrefixBase
	}
}

func readResult(path string) (perf.RunResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return perf.RunResult{}, err
	}
	defer f.Close()
	return perf.DecodeResult(f)
}

func writeJSON(path string, result perf.RunResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return perf.MarshalJSONDeterministic(result, f)
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func dirtyWorktree() bool {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	return err != nil || strings.TrimSpace(string(out)) != ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstEnv(keys ...string) string {
	fallback := ""
	if len(keys) > 0 {
		fallback = keys[len(keys)-1]
		keys = keys[:len(keys)-1]
	}
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes"
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: s3s5-perf run|report|validate|baseline|scenarios|version")
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "s3s5-perf: %v\n", err)
	os.Exit(1)
}
