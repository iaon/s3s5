package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"

	"s3s5/internal/config"
	s3crypto "s3s5/internal/crypto"
	s3store "s3s5/internal/objectstore/s3"
	"s3s5/internal/socks5"
	"s3s5/internal/tunnel"
	"s3s5/internal/version"
)

func main() {
	var common config.Common
	var listen string
	var showVersion bool
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	config.AddCommonFlags(fs, &common)
	fs.StringVar(&listen, "listen", "127.0.0.1:1080", "local SOCKS5 listen address")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	_ = fs.Parse(os.Args[1:])
	if showVersion {
		fmt.Fprintf(os.Stdout, "s3s5-client %s\n", version.String())
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
	stats := &tunnel.Stats{}
	client, err := tunnel.NewClient(tunnel.Config{
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
	})
	if err != nil {
		fatal(err)
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		fatal(err)
	}
	defer ln.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Fprintf(os.Stderr, "s3s5-client listening on %s\n", listen)
	err = (&socks5.Server{Handler: client.HandleSOCKS}).Serve(ctx, ln)
	if err != nil && ctx.Err() == nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "s3s5-client stats: %+v\n", stats.Snapshot())
}

func buildCodec(c config.Common) (s3crypto.Codec, error) {
	if c.InsecureNoCrypto {
		fmt.Fprintln(os.Stderr, "WARNING: --insecure-no-crypto disables payload encryption; use only for local development")
		return s3crypto.NoopCodec{}, nil
	}
	return s3crypto.NewPSKCodec(c.PSK)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "s3s5-client: %v\n", err)
	os.Exit(1)
}
