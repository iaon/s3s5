package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Common struct {
	Provider         string
	Bucket           string
	Prefix           string
	Region           string
	Endpoint         string
	ForcePathStyle   bool
	PSKEnv           string
	PSK              string
	ChunkSize        int
	PollMin          time.Duration
	PollMax          time.Duration
	WindowChunks     uint64
	IdleTimeout      time.Duration
	LogLevel         string
	InsecureNoCrypto bool
}

func AddCommonFlags(fs *flag.FlagSet, c *Common) {
	fs.StringVar(&c.Provider, "provider", getenv("S3S5_PROVIDER", "aws"), "S3 provider preset: aws, yandex, minio, custom")
	fs.StringVar(&c.Bucket, "bucket", getenv("S3S5_BUCKET", ""), "S3 bucket name")
	fs.StringVar(&c.Prefix, "prefix", getenv("S3S5_PREFIX", "s3s5"), "S3 key prefix")
	fs.StringVar(&c.Region, "region", firstEnv("S3S5_REGION", "AWS_REGION", "us-east-1"), "S3 region")
	fs.StringVar(&c.Endpoint, "endpoint", getenv("S3S5_ENDPOINT", ""), "S3-compatible endpoint URL")
	fs.BoolVar(&c.ForcePathStyle, "force-path-style", getenvBool("S3S5_FORCE_PATH_STYLE", false), "use path-style S3 URLs")
	fs.StringVar(&c.PSKEnv, "psk-env", "S3S5_PSK", "environment variable containing the pre-shared key")
	fs.IntVar(&c.ChunkSize, "chunk-size", getenvInt("S3S5_CHUNK_SIZE", 64*1024), "data chunk size in bytes")
	fs.DurationVar(&c.PollMin, "poll-min", getenvDuration("S3S5_POLL_MIN", 50*time.Millisecond), "minimum polling delay")
	fs.DurationVar(&c.PollMax, "poll-max", getenvDuration("S3S5_POLL_MAX", 2*time.Second), "maximum polling delay")
	fs.Uint64Var(&c.WindowChunks, "window-chunks", getenvUint64("S3S5_WINDOW_CHUNKS", 16), "max unacknowledged chunks per direction")
	fs.DurationVar(&c.IdleTimeout, "idle-timeout", getenvDuration("S3S5_IDLE_TIMEOUT", 2*time.Minute), "idle session timeout")
	fs.StringVar(&c.LogLevel, "log-level", getenv("S3S5_LOG_LEVEL", "info"), "log level")
	fs.BoolVar(&c.InsecureNoCrypto, "insecure-no-crypto", false, "disable payload encryption for local development only")
}

func (c *Common) Finalize(requireBucket bool) error {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	if c.Provider == "" {
		c.Provider = "aws"
	}
	c.applyProviderDefaults()
	c.Prefix = strings.Trim(c.Prefix, "/")
	if c.Prefix == "" {
		c.Prefix = "s3s5"
	}
	if requireBucket && c.Bucket == "" {
		return errors.New("--bucket or S3S5_BUCKET is required")
	}
	if c.ChunkSize <= 0 {
		return errors.New("--chunk-size must be positive")
	}
	if c.WindowChunks == 0 {
		return errors.New("--window-chunks must be positive")
	}
	if c.PollMin <= 0 || c.PollMax <= 0 || c.PollMin > c.PollMax {
		return errors.New("poll delays must be positive and poll-min must be <= poll-max")
	}
	if !c.InsecureNoCrypto {
		c.PSK = os.Getenv(c.PSKEnv)
		if c.PSK == "" {
			return fmt.Errorf("%s is required unless --insecure-no-crypto is used", c.PSKEnv)
		}
	}
	return nil
}

func (c *Common) applyProviderDefaults() {
	switch c.Provider {
	case "yandex", "yc", "yandex-cloud", "yandexcloud":
		c.Provider = "yandex"
		if c.Endpoint == "" {
			c.Endpoint = "https://storage.yandexcloud.net"
		}
		if c.Region == "" || c.Region == "us-east-1" || strings.HasPrefix(c.Region, "ru-central1-") {
			c.Region = "ru-central1"
		}
		c.ForcePathStyle = true
	case "minio":
		if c.Endpoint == "" {
			c.Endpoint = "http://127.0.0.1:9000"
		}
		if c.Region == "" {
			c.Region = "us-east-1"
		}
		c.ForcePathStyle = true
	case "custom":
		if c.Endpoint == "" {
			// Leave endpoint validation to the S3 store so tests can construct partial configs.
			return
		}
	case "aws":
		if c.Region == "" {
			c.Region = "us-east-1"
		}
	}
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

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvUint64(key string, fallback uint64) uint64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
