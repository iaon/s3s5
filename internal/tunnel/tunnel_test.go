package tunnel

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	s3crypto "s3s5/internal/crypto"
	"s3s5/internal/objectstore"
	"s3s5/internal/objectstore/memory"
	"s3s5/internal/policy"
	"s3s5/internal/protocol"
	"s3s5/internal/socks5"
)

func TestMemoryStoreSOCKS5RoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := memory.New()
	codec, err := s3crypto.NewPSKCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(store, codec)
	pol, err := policy.New(policy.Config{AllowPrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(cfg, pol)
	if err != nil {
		t.Fatal(err)
	}
	server.ConnectTimeout = time.Second
	go func() { _ = server.Run(ctx) }()

	echoAddr, stopEcho := startEcho(t)
	defer stopEcho()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	socksAddr, stopSOCKS := startSOCKS(t, ctx, client)
	defer stopSOCKS()

	conn, err := net.Dial("tcp", socksAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := socksHandshake(conn, echoAddr); err != nil {
		t.Fatal(err)
	}
	msg := []byte("hello through s3 mailbox")
	if _, err := conn.Write(msg); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(msg) {
		t.Fatalf("echo mismatch: %q", got)
	}
}

func TestMemoryStoreSOCKS5PolicyDenial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := memory.New()
	cfg := testConfig(store, s3crypto.NoopCodec{})
	pol, err := policy.New(policy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(cfg, pol)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Run(ctx) }()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	socksAddr, stopSOCKS := startSOCKS(t, ctx, client)
	defer stopSOCKS()

	conn, err := net.Dial("tcp", socksAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reply, err := rawSOCKSConnect(conn, "127.0.0.1:9")
	if err != nil {
		t.Fatal(err)
	}
	if reply == socks5.ReplySucceeded {
		t.Fatal("expected loopback target to be denied by default")
	}
}

func TestNewClientAndServerRequireExplicitCodec(t *testing.T) {
	store := memory.New()
	if _, err := NewClient(testConfig(store, nil)); err == nil {
		t.Fatal("expected nil codec to be rejected for client")
	}
	if _, err := NewServer(testConfig(store, nil), nil); err == nil {
		t.Fatal("expected nil codec to be rejected for server")
	}
	cfg := testConfig(store, s3crypto.NoopCodec{})
	pol, err := policy.New(policy.Config{RequireAllowTarget: true, AllowUnrestricted: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient(cfg); err != nil {
		t.Fatalf("explicit NoopCodec should be accepted for client: %v", err)
	}
	if _, err := NewServer(cfg, pol); err != nil {
		t.Fatalf("explicit NoopCodec should be accepted for server: %v", err)
	}
}

func TestByteLimitAppliesInBothDirections(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	cfg := testConfig(store, s3crypto.NoopCodec{})
	sessionID := "session-byte-limit"

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.streamToStore(ctx, sessionID, DirectionC2S, SideClient, strings.NewReader("abcdef"), 4); err == nil {
		t.Fatal("expected client upload stream to stop at max bytes")
	}
	if _, err := store.HeadObject(ctx, protocol.CloseKey(cfg.Prefix, SideClient, sessionID)); err != nil {
		t.Fatalf("expected client close marker after byte limit: %v", err)
	}

	if err := store.PutObject(ctx, protocol.DataKey(cfg.Prefix, DirectionS2C, sessionID, 0), []byte("abcdef"), objectstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.streamFromStore(ctx, sessionID, DirectionS2C, SideClient, &bytes.Buffer{}, 4); err == nil {
		t.Fatal("expected client download stream to stop at max bytes")
	}
}

func testConfig(store *memory.Store, codec s3crypto.Codec) Config {
	return Config{
		Store:        store,
		Codec:        codec,
		Prefix:       "test",
		ChunkSize:    8,
		PollMin:      time.Millisecond,
		PollMax:      10 * time.Millisecond,
		WindowChunks: 2,
		IdleTimeout:  3 * time.Second,
	}
}

func startSOCKS(t *testing.T, ctx context.Context, client *Client) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = (&socks5.Server{Handler: client.HandleSOCKS}).Serve(ctx, ln)
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func startEcho(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
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
	}
}

func socksHandshake(conn net.Conn, target string) error {
	reply, err := rawSOCKSConnect(conn, target)
	if err != nil {
		return err
	}
	if reply != socks5.ReplySucceeded {
		return strconv.ErrSyntax
	}
	return nil
}

func rawSOCKSConnect(conn net.Conn, target string) (byte, error) {
	if _, err := conn.Write([]byte{socks5.Version5, 1, socks5.MethodNoAuth}); err != nil {
		return 0, err
	}
	method := make([]byte, 2)
	if _, err := io.ReadFull(conn, method); err != nil {
		return 0, err
	}
	host, portRaw, err := net.SplitHostPort(target)
	if err != nil {
		return 0, err
	}
	port64, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		return 0, err
	}
	ip := net.ParseIP(host).To4()
	req := []byte{socks5.Version5, socks5.CmdConnect, 0, socks5.AtypIPv4}
	req = append(req, ip...)
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port64))
	req = append(req, p[:]...)
	if _, err := conn.Write(req); err != nil {
		return 0, err
	}
	resp := make([]byte, 10)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return 0, err
	}
	return resp[1], nil
}
