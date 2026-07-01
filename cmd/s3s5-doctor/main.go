package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"s3s5/internal/config"
	s3crypto "s3s5/internal/crypto"
	"s3s5/internal/objectstore"
	s3store "s3s5/internal/objectstore/s3"
	"s3s5/internal/protocol"
)

type report struct {
	Bucket   string        `json:"bucket"`
	Prefix   string        `json:"prefix"`
	Endpoint string        `json:"endpoint,omitempty"`
	Crypto   string        `json:"crypto"`
	Latency  time.Duration `json:"latency"`
	Rounds   int           `json:"rounds"`
	PutOK    bool          `json:"put_ok"`
	HeadOK   bool          `json:"head_ok"`
	GetOK    bool          `json:"get_ok"`
	ListOK   bool          `json:"list_ok"`
	DeleteOK bool          `json:"delete_ok"`
	Error    string        `json:"error,omitempty"`
}

func main() {
	var common config.Common
	var cleanup bool
	var cleanupPrefix bool
	var rounds int
	var jsonOut bool
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	config.AddCommonFlags(fs, &common)
	fs.BoolVar(&cleanup, "cleanup", true, "delete doctor test objects")
	fs.BoolVar(&cleanupPrefix, "cleanup-prefix", false, "delete all objects under --prefix and exit")
	fs.IntVar(&rounds, "latency-rounds", 3, "round-trip latency rounds")
	fs.BoolVar(&jsonOut, "json", false, "emit JSON")
	_ = fs.Parse(os.Args[1:])
	if err := common.Finalize(true); err != nil {
		fatal(err)
	}
	codec, err := buildCodec(common)
	if err != nil {
		fatal(err)
	}
	store, err := s3store.New(s3store.Config{
		Bucket:         common.Bucket,
		Region:         common.Region,
		Endpoint:       common.Endpoint,
		ForcePathStyle: common.ForcePathStyle,
	})
	if err != nil {
		fatal(err)
	}
	if cleanupPrefix {
		deleted, err := objectstore.DeletePrefix(context.Background(), store, protocol.NormalizePrefix(common.Prefix)+"/")
		if err != nil {
			fatal(err)
		}
		if jsonOut {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"deleted": deleted, "prefix": protocol.NormalizePrefix(common.Prefix)})
		} else {
			fmt.Fprintf(os.Stdout, "deleted %d objects under prefix %s\n", deleted, protocol.NormalizePrefix(common.Prefix))
		}
		return
	}
	rep := runDoctor(context.Background(), common, codec, store, cleanup, rounds)
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		if rep.Error != "" {
			fmt.Fprintf(os.Stdout, "doctor failed: %s\n", rep.Error)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "doctor ok: bucket=%s prefix=%s crypto=%s avg_latency=%s rounds=%d\n", rep.Bucket, rep.Prefix, rep.Crypto, rep.Latency, rep.Rounds)
	}
	if rep.Error != "" {
		os.Exit(1)
	}
}

func runDoctor(ctx context.Context, c config.Common, codec s3crypto.Codec, store objectstore.ObjectStore, cleanup bool, rounds int) report {
	if rounds <= 0 {
		rounds = 1
	}
	cryptoName := "psk-aes-256-gcm"
	if !codec.Enabled() {
		cryptoName = "disabled"
	}
	rep := report{Bucket: c.Bucket, Prefix: c.Prefix, Endpoint: c.Endpoint, Crypto: cryptoName, Rounds: rounds}
	sessionID, err := protocol.NewSessionID()
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	key := protocol.NormalizePrefix(c.Prefix) + "/doctor/" + sessionID + ".bin"
	var total time.Duration
	for i := 0; i < rounds; i++ {
		payload := []byte(fmt.Sprintf("s3s5 doctor round %d", i))
		sealed, err := codec.Seal("doctor", sessionID, "control", uint64(i), payload)
		if err != nil {
			rep.Error = err.Error()
			return rep
		}
		start := time.Now()
		if err := store.PutObject(ctx, key, sealed, objectstore.PutOptions{ContentType: "application/octet-stream"}); err != nil {
			rep.Error = err.Error()
			return rep
		}
		rep.PutOK = true
		if _, err := store.HeadObject(ctx, key); err != nil {
			rep.Error = err.Error()
			return rep
		}
		rep.HeadOK = true
		got, err := store.GetObject(ctx, key)
		if err != nil {
			rep.Error = err.Error()
			return rep
		}
		plain, err := codec.Open("doctor", sessionID, "control", uint64(i), got)
		if err != nil {
			rep.Error = err.Error()
			return rep
		}
		if string(plain) != string(payload) {
			rep.Error = "doctor payload mismatch"
			return rep
		}
		rep.GetOK = true
		if _, err := store.ListPrefix(ctx, protocol.NormalizePrefix(c.Prefix)+"/doctor/", objectstore.ListOptions{MaxKeys: 10}); err != nil {
			rep.Error = err.Error()
			return rep
		}
		rep.ListOK = true
		total += time.Since(start)
	}
	if cleanup {
		if err := store.DeleteObject(ctx, key); err != nil {
			rep.Error = err.Error()
			return rep
		}
		rep.DeleteOK = true
	}
	rep.Latency = total / time.Duration(rounds)
	return rep
}

func buildCodec(c config.Common) (s3crypto.Codec, error) {
	if c.InsecureNoCrypto {
		fmt.Fprintln(os.Stderr, "WARNING: --insecure-no-crypto disables payload encryption; use only for local development")
		return s3crypto.NoopCodec{}, nil
	}
	return s3crypto.NewPSKCodec(c.PSK)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "s3s5-doctor: %v\n", err)
	os.Exit(1)
}
