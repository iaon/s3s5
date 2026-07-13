package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"s3s5/internal/config"
	s3crypto "s3s5/internal/crypto"
	s3store "s3s5/internal/objectstore/s3"
	"s3s5/internal/policy"
	"s3s5/internal/tunnel"
	"s3s5/internal/version"
)

type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() {
	var common config.Common
	var allowTargets, denyTargets multiFlag
	var allowPrivate bool
	var allowUnrestricted bool
	var maxSessions int
	var maxBytes int64
	var connectTimeout time.Duration
	var showVersion bool
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	config.AddCommonFlags(fs, &common)
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.Var(&allowTargets, "allow-target", "required allow target rule unless --allow-unrestricted-egress is set; repeatable; examples: example.com:443, *.example.com, 203.0.113.0/24")
	fs.Var(&denyTargets, "deny-target", "deny target rule, repeatable")
	fs.BoolVar(&allowPrivate, "allow-private", false, "allow private, loopback, link-local, multicast, unspecified, and metadata ranges")
	fs.BoolVar(&allowUnrestricted, "allow-unrestricted-egress", false, "allow unrestricted public egress; safer deployments should use --allow-target")
	fs.IntVar(&maxSessions, "max-sessions", 32, "maximum concurrent sessions")
	fs.Int64Var(&maxBytes, "max-bytes-per-session", 1<<30, "maximum bytes sent from server to client per session")
	fs.DurationVar(&connectTimeout, "connect-timeout", 10*time.Second, "outbound target connect timeout")
	_ = fs.Parse(os.Args[1:])
	if showVersion {
		fmt.Fprintf(os.Stdout, "s3s5-server %s\n", version.String())
		return
	}
	if err := common.Finalize(true); err != nil {
		fatal(err)
	}
	codec, err := buildCodec(common)
	if err != nil {
		fatal(err)
	}
	store, err := s3store.New(s3store.Config{
		Provider:       common.Provider,
		Bucket:         common.Bucket,
		Region:         common.Region,
		Endpoint:       common.Endpoint,
		ForcePathStyle: common.ForcePathStyle,
	})
	if err != nil {
		fatal(err)
	}
	pol, err := policy.New(policy.Config{
		AllowPrivate:       allowPrivate,
		AllowUnrestricted:  allowUnrestricted,
		RequireAllowTarget: true,
		AllowTargets:       allowTargets,
		DenyTargets:        denyTargets,
	})
	if err != nil {
		fatal(err)
	}
	stats := &tunnel.Stats{}
	server, err := tunnel.NewServer(tunnel.Config{
		Store:                 store,
		Codec:                 codec,
		Stats:                 stats,
		Prefix:                common.Prefix,
		ChunkSize:             common.ChunkSize,
		FlushDelay:            common.FlushDelay,
		PollMin:               common.PollMin,
		PollMax:               common.PollMax,
		ActivePollDuration:    common.ActivePollDuration,
		WindowChunks:          common.WindowChunks,
		CloseCheckAfterMisses: common.CloseCheckAfterMisses,
		IdleTimeout:           common.IdleTimeout,
	}, pol)
	if err != nil {
		fatal(err)
	}
	server.ConnectTimeout = connectTimeout
	server.MaxSessions = maxSessions
	server.MaxBytesPerSession = maxBytes
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Fprintf(os.Stderr, "s3s5-server polling bucket=%s prefix=%s\n", common.Bucket, common.Prefix)
	if err := server.Run(ctx); err != nil && ctx.Err() == nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "s3s5-server stats: %+v\n", stats.Snapshot())
}

func buildCodec(c config.Common) (s3crypto.Codec, error) {
	if c.InsecureNoCrypto {
		fmt.Fprintln(os.Stderr, "WARNING: --insecure-no-crypto disables payload encryption; use only for local development")
		return s3crypto.NoopCodec{}, nil
	}
	return s3crypto.NewPSKCodec(c.PSK)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "s3s5-server: %v\n", err)
	os.Exit(1)
}
