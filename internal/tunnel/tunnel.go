package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	s3crypto "s3s5/internal/crypto"
	"s3s5/internal/objectstore"
	"s3s5/internal/policy"
	"s3s5/internal/protocol"
	"s3s5/internal/socks5"
)

const (
	DirectionC2S = "c2s"
	DirectionS2C = "s2c"
	SideClient   = "client"
	SideServer   = "server"
)

type Config struct {
	Store                 objectstore.ObjectStore
	Codec                 s3crypto.Codec
	Stats                 *Stats
	Prefix                string
	ChunkSize             int
	FlushDelay            time.Duration
	PollMin               time.Duration
	PollMax               time.Duration
	ActivePollDuration    time.Duration
	WindowChunks          uint64
	CloseCheckAfterMisses int
	IdleTimeout           time.Duration
}

type Client struct {
	cfg Config
}

type Server struct {
	cfg                Config
	Policy             *policy.Engine
	ConnectTimeout     time.Duration
	MaxSessions        int
	MaxBytesPerSession int64

	inFlight sync.Map
}

type SendWindow struct {
	AckedNextSeq uint64
}

const (
	defaultCloseCheckAfterMisses = 4
	defaultActivePollDuration    = 500 * time.Millisecond
)

func NewClient(cfg Config) (*Client, error) {
	cfg = withDefaults(cfg)
	if cfg.Store == nil {
		return nil, errors.New("object store is required")
	}
	if cfg.Codec == nil {
		return nil, errors.New("codec is required; pass NoopCodec explicitly only for local insecure tests")
	}
	return &Client{cfg: cfg}, nil
}

func NewServer(cfg Config, pol *policy.Engine) (*Server, error) {
	cfg = withDefaults(cfg)
	if cfg.Store == nil {
		return nil, errors.New("object store is required")
	}
	if cfg.Codec == nil {
		return nil, errors.New("codec is required; pass NoopCodec explicitly only for local insecure tests")
	}
	if pol == nil {
		var err error
		pol, err = policy.New(policy.Config{})
		if err != nil {
			return nil, err
		}
	}
	return &Server{cfg: cfg, Policy: pol, ConnectTimeout: 10 * time.Second, MaxSessions: 32, MaxBytesPerSession: 1 << 30}, nil
}

func (c *Client) HandleSOCKS(ctx context.Context, target protocol.Target, conn net.Conn, reply func(byte) error) error {
	startedAt := time.Now()
	sessionID, err := protocol.NewSessionID()
	if err != nil {
		_ = reply(socks5.ReplyGeneralFailure)
		return err
	}
	c.cfg.Stats.startSession(sessionID, startedAt)
	req := protocol.OpenRequest{
		Version:             protocol.Version,
		SessionID:           sessionID,
		Target:              target,
		MaxReceiveChunkSize: c.cfg.ChunkSize,
		CreatedAt:           time.Now().UTC(),
	}
	if err := c.putJSON(ctx, protocol.OpenKey(c.cfg.Prefix, sessionID), "open", sessionID, "control", req); err != nil {
		c.cfg.Stats.finishSession(sessionID, time.Since(startedAt), err)
		_ = reply(socks5.ReplyGeneralFailure)
		return err
	}
	var result protocol.OpenResult
	if err := c.waitJSON(ctx, protocol.OpenResultKey(c.cfg.Prefix, sessionID), "open-result", sessionID, "control", &result); err != nil {
		c.cfg.Stats.finishSession(sessionID, time.Since(startedAt), err)
		_ = reply(socks5.ReplyHostUnreachable)
		return err
	}
	if !result.Accepted {
		c.cfg.Stats.rejectSession(sessionID, time.Since(startedAt))
		_ = reply(socks5.ReplyHostUnreachable)
		return fmt.Errorf("server rejected target: %s", result.Error)
	}
	c2sMax, err := protocol.EffectiveSendChunkSize(c.cfg.ChunkSize, result.MaxReceiveChunkSize)
	if err != nil {
		c.cfg.Stats.finishSession(sessionID, time.Since(startedAt), err)
		_ = reply(socks5.ReplyHostUnreachable)
		return err
	}
	sessionCfg := c.cfg
	sessionCfg.ChunkSize = c2sMax
	c.cfg.Stats.openSession(time.Since(startedAt))
	if err := reply(socks5.ReplySucceeded); err != nil {
		c.cfg.Stats.finishSession(sessionID, time.Since(startedAt), err)
		return err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	activity := newActivitySignal()
	errCh := make(chan error, 2)
	go func() {
		errCh <- streamToStore(sessionCtx, sessionCfg, sessionID, DirectionC2S, SideClient, conn, 0, activity)
	}()
	go func() {
		err := streamFromStore(sessionCtx, sessionCfg, sessionID, DirectionS2C, SideServer, conn, 0, activity)
		closeWrite(conn)
		errCh <- err
	}()
	var first error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && first == nil && !isClosedConn(err) {
			first = err
			cancel()
		}
	}
	c.cfg.Stats.finishSession(sessionID, time.Since(startedAt), first)
	return first
}

func (s *Server) Run(ctx context.Context) error {
	sem := make(chan struct{}, max(1, s.MaxSessions))
	delay := s.cfg.PollMin
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		keys, err := s.listOpenKeys(ctx)
		if err != nil {
			return err
		}
		started := 0
		for _, key := range keys {
			sessionID := strings.TrimSuffix(path.Base(key), ".json")
			if sessionID == "" {
				continue
			}
			if _, loaded := s.inFlight.LoadOrStore(sessionID, struct{}{}); loaded {
				continue
			}
			started++
			sem <- struct{}{}
			go func(id string) {
				defer func() {
					s.inFlight.Delete(id)
					<-sem
				}()
				s.cfg.Stats.incActive()
				defer s.cfg.Stats.decActive()
				_ = s.handleSession(ctx, id)
			}(sessionID)
		}
		if started > 0 {
			delay = s.cfg.PollMin
		} else {
			delay = nextDelay(delay, s.cfg.PollMax)
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
	}
}

func (s *Server) listOpenKeys(ctx context.Context) ([]string, error) {
	opts := objectstore.ListOptions{MaxKeys: 1000}
	var keys []string
	for {
		s.cfg.Stats.incList()
		page, err := s.cfg.Store.ListPrefixPage(ctx, protocol.OpenPrefix(s.cfg.Prefix), opts)
		if err != nil {
			return nil, err
		}
		keys = append(keys, page.Keys...)
		if !page.IsTruncated || page.NextContinuationToken == "" {
			return keys, nil
		}
		opts.ContinuationToken = page.NextContinuationToken
	}
}

func (s *Server) handleSession(ctx context.Context, sessionID string) error {
	var req protocol.OpenRequest
	if err := s.getJSON(ctx, protocol.OpenKey(s.cfg.Prefix, sessionID), "open", sessionID, "control", &req); err != nil {
		return err
	}
	if req.Version != protocol.Version || req.SessionID != sessionID || protocol.ValidateChunkSize(req.MaxReceiveChunkSize) != nil {
		err := s.putOpenResult(ctx, sessionID, false, "invalid open request")
		s.cfg.Stats.incDelete()
		_ = s.cfg.Store.DeleteObject(context.Background(), protocol.OpenKey(s.cfg.Prefix, sessionID))
		return err
	}
	s.cfg.Stats.incDelete()
	_ = s.cfg.Store.DeleteObject(context.Background(), protocol.OpenKey(s.cfg.Prefix, sessionID))
	s2cMax, err := protocol.EffectiveSendChunkSize(s.cfg.ChunkSize, req.MaxReceiveChunkSize)
	if err != nil {
		return s.putOpenResult(ctx, sessionID, false, "invalid receive chunk limit")
	}
	dialCtx, cancel := context.WithTimeout(ctx, s.ConnectTimeout)
	defer cancel()
	conn, err := s.dialTarget(dialCtx, req.Target)
	if err != nil {
		return s.putOpenResult(ctx, sessionID, false, err.Error())
	}
	defer conn.Close()
	if err := s.putOpenResult(ctx, sessionID, true, ""); err != nil {
		return err
	}
	sessionCtx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()
	sessionCfg := s.cfg
	sessionCfg.ChunkSize = s2cMax
	activity := newActivitySignal()
	errCh := make(chan error, 2)
	go func() {
		err := streamFromStore(sessionCtx, sessionCfg, sessionID, DirectionC2S, SideClient, conn, s.MaxBytesPerSession, activity)
		closeWrite(conn)
		errCh <- err
	}()
	go func() {
		errCh <- streamToStore(sessionCtx, sessionCfg, sessionID, DirectionS2C, SideServer, conn, s.MaxBytesPerSession, activity)
	}()
	var first error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && first == nil && !isClosedConn(err) {
			first = err
			cancelSession()
		}
	}
	return first
}

func (s *Server) dialTarget(ctx context.Context, target protocol.Target) (net.Conn, error) {
	if s.Policy != nil {
		ips, err := s.Policy.AllowedIPs(ctx, target, net.DefaultResolver)
		if err != nil {
			return nil, err
		}
		var first error
		d := net.Dialer{Timeout: s.ConnectTimeout}
		for _, ip := range ips {
			conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(int(target.Port))))
			if err == nil {
				return conn, nil
			}
			if first == nil {
				first = err
			}
		}
		return nil, first
	}
	d := net.Dialer{Timeout: s.ConnectTimeout}
	return d.DialContext(ctx, "tcp", net.JoinHostPort(target.Host, strconv.Itoa(int(target.Port))))
}

func (s *Server) putOpenResult(ctx context.Context, sessionID string, accepted bool, msg string) error {
	res := protocol.OpenResult{
		Version:             protocol.Version,
		SessionID:           sessionID,
		Accepted:            accepted,
		Error:               msg,
		MaxReceiveChunkSize: s.cfg.ChunkSize,
		CreatedAt:           time.Now().UTC(),
	}
	return s.putJSON(ctx, protocol.OpenResultKey(s.cfg.Prefix, sessionID), "open-result", sessionID, "control", res)
}

func (c *Client) putJSON(ctx context.Context, key, typ, sessionID, direction string, v any) error {
	return putJSON(ctx, c.cfg, key, typ, sessionID, direction, v)
}

func (s *Server) putJSON(ctx context.Context, key, typ, sessionID, direction string, v any) error {
	return putJSON(ctx, s.cfg, key, typ, sessionID, direction, v)
}

func (s *Server) getJSON(ctx context.Context, key, typ, sessionID, direction string, v any) error {
	return getJSON(ctx, s.cfg, key, typ, sessionID, direction, v)
}

func (c *Client) waitJSON(ctx context.Context, key, typ, sessionID, direction string, v any) error {
	return waitJSON(ctx, c.cfg, key, typ, sessionID, direction, v)
}

func putJSON(ctx context.Context, cfg Config, key, typ, sessionID, direction string, v any) error {
	data, err := protocol.Marshal(v)
	if err != nil {
		return err
	}
	sealed, err := cfg.Codec.Seal(typ, sessionID, direction, 0, data)
	if err != nil {
		return err
	}
	cfg.Stats.incPut()
	return cfg.Store.PutObject(ctx, key, sealed, objectstore.PutOptions{ContentType: "application/octet-stream"})
}

func getJSON(ctx context.Context, cfg Config, key, typ, sessionID, direction string, v any) error {
	cfg.Stats.incGet()
	data, err := cfg.Store.GetObject(ctx, key)
	if err != nil {
		return err
	}
	plain, err := cfg.Codec.Open(typ, sessionID, direction, 0, data)
	if err != nil {
		return err
	}
	return protocol.Unmarshal(plain, v)
}

func waitJSON(ctx context.Context, cfg Config, key, typ, sessionID, direction string, v any) error {
	deadline := time.Now().Add(cfg.IdleTimeout)
	delay := cfg.PollMin
	for {
		err := getJSON(ctx, cfg, key, typ, sessionID, direction, v)
		if err == nil {
			return nil
		}
		if !objectstore.IsNotFound(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s", key)
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
		delay = nextDelay(delay, cfg.PollMax)
	}
}

func (c *Client) streamToStore(ctx context.Context, sessionID, direction, side string, r io.Reader, maxBytes int64) error {
	return streamToStore(ctx, c.cfg, sessionID, direction, side, r, maxBytes, nil)
}

func (s *Server) streamToStore(ctx context.Context, sessionID, direction, side string, r io.Reader, maxBytes int64) error {
	return streamToStore(ctx, s.cfg, sessionID, direction, side, r, maxBytes, nil)
}

func streamToStore(ctx context.Context, cfg Config, sessionID, direction, side string, r io.Reader, maxBytes int64, activity *activitySignal) error {
	aggregator := Aggregator{MaxBytes: cfg.ChunkSize, FlushDelay: cfg.FlushDelay}
	window := &SendWindow{}
	var seq uint64
	var sent int64
	for {
		read, readErr := aggregator.Read(ctx, r)
		if len(read.Data) > 0 {
			cfg.Stats.recordAggregation(read.Reason, read.SocketReads)
			notifyActivity(activity)
			if maxBytes > 0 && sent+int64(len(read.Data)) > maxBytes {
				_ = putClose(ctx, cfg, sessionID, side, "max bytes per session exceeded")
				return fmt.Errorf("max bytes per session exceeded")
			}
			if err := waitWindow(ctx, cfg, sessionID, direction, seq, window); err != nil {
				return err
			}
			sealed, err := cfg.Codec.SealData(sessionID, direction, seq, read.Data)
			if err != nil {
				return err
			}
			cfg.Stats.incPut()
			if err := cfg.Store.PutObject(ctx, protocol.DataKey(cfg.Prefix, direction, sessionID, seq), sealed, objectstore.PutOptions{ContentType: "application/octet-stream"}); err != nil {
				return err
			}
			notifyActivity(activity)
			if seq == 0 {
				cfg.Stats.recordFirstData(sessionID, direction, time.Now())
			}
			cfg.Stats.incChunksSent(len(read.Data), len(sealed))
			seq++
			sent += int64(len(read.Data))
		}
		if readErr == io.EOF {
			return putClose(ctx, cfg, sessionID, side, "")
		}
		if readErr != nil {
			_ = putClose(ctx, cfg, sessionID, side, readErr.Error())
			return readErr
		}
	}
}

func (c *Client) streamFromStore(ctx context.Context, sessionID, direction, peerSide string, w io.Writer, maxBytes int64) error {
	return streamFromStore(ctx, c.cfg, sessionID, direction, peerSide, w, maxBytes, nil)
}

func (s *Server) streamFromStore(ctx context.Context, sessionID, direction, peerSide string, w io.Writer, maxBytes int64) error {
	return streamFromStore(ctx, s.cfg, sessionID, direction, peerSide, w, maxBytes, nil)
}

func streamFromStore(ctx context.Context, cfg Config, sessionID, direction, peerSide string, w io.Writer, maxBytes int64, activity *activitySignal) error {
	var seq uint64
	var lastAck uint64
	var received int64
	delay := cfg.PollMin
	deadline := time.Now().Add(cfg.IdleTimeout)
	closeCheckEvery := cfg.CloseCheckAfterMisses
	if closeCheckEvery <= 0 {
		closeCheckEvery = defaultCloseCheckAfterMisses
	}
	missesSinceCloseCheck := 0
	ackEvery := ackInterval(cfg.WindowChunks)
	for {
		key := protocol.DataKey(cfg.Prefix, direction, sessionID, seq)
		cfg.Stats.incGet()
		data, err := cfg.Store.GetObject(ctx, key)
		if err == nil {
			plain, err := cfg.Codec.OpenData(sessionID, direction, seq, data)
			if err != nil {
				return err
			}
			if maxBytes > 0 && received+int64(len(plain)) > maxBytes {
				return fmt.Errorf("max bytes per session exceeded")
			}
			if _, err := w.Write(plain); err != nil {
				return err
			}
			cfg.Stats.incChunksReceived(len(plain), len(data))
			notifyActivity(activity)
			received += int64(len(plain))
			seq++
			if seq-lastAck >= ackEvery {
				if err := putAck(ctx, cfg, sessionID, direction, seq); err != nil {
					return err
				}
				lastAck = seq
				notifyActivity(activity)
			}
			delay = cfg.PollMin
			deadline = time.Now().Add(cfg.IdleTimeout)
			missesSinceCloseCheck = 0
			continue
		}
		if !objectstore.IsNotFound(err) {
			return err
		}
		missesSinceCloseCheck++
		if missesSinceCloseCheck >= closeCheckEvery {
			missesSinceCloseCheck = 0
			if closed, err := closeExists(ctx, cfg, sessionID, peerSide); err != nil {
				return err
			} else if closed {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("idle timeout waiting for %s seq %d", direction, seq)
		}
		woke, err := sleepOrActivity(ctx, delay, cfg.ActivePollDuration, activity)
		if err != nil {
			return err
		}
		if woke {
			delay = cfg.PollMin
		} else {
			delay = nextDelay(delay, cfg.PollMax)
		}
	}
}

func waitWindow(ctx context.Context, cfg Config, sessionID, direction string, seq uint64, window *SendWindow) error {
	if cfg.WindowChunks == 0 {
		return nil
	}
	if window == nil {
		window = &SendWindow{}
	}
	if seq < window.AckedNextSeq+cfg.WindowChunks {
		return nil
	}
	delay := cfg.PollMin
	for {
		next, err := getAck(ctx, cfg, sessionID, direction)
		if err != nil {
			return err
		}
		if next > window.AckedNextSeq {
			window.AckedNextSeq = next
		}
		if seq < window.AckedNextSeq+cfg.WindowChunks {
			return nil
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
		delay = nextDelay(delay, cfg.PollMax)
	}
}

func ackInterval(window uint64) uint64 {
	if window <= 2 {
		return 1
	}
	return window / 2
}

func putAck(ctx context.Context, cfg Config, sessionID, direction string, next uint64) error {
	ack := protocol.Ack{
		Version:   protocol.Version,
		SessionID: sessionID,
		Direction: direction,
		NextSeq:   next,
		UpdatedAt: time.Now().UTC(),
	}
	return putJSON(ctx, cfg, protocol.AckKey(cfg.Prefix, direction, sessionID), "ack", sessionID, direction, ack)
}

func getAck(ctx context.Context, cfg Config, sessionID, direction string) (uint64, error) {
	var ack protocol.Ack
	err := getJSON(ctx, cfg, protocol.AckKey(cfg.Prefix, direction, sessionID), "ack", sessionID, direction, &ack)
	if objectstore.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return ack.NextSeq, nil
}

func putClose(ctx context.Context, cfg Config, sessionID, side, reason string) error {
	msg := protocol.Close{
		Version:   protocol.Version,
		SessionID: sessionID,
		Side:      side,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	}
	return putJSON(ctx, cfg, protocol.CloseKey(cfg.Prefix, side, sessionID), "close", sessionID, side, msg)
}

func closeExists(ctx context.Context, cfg Config, sessionID, side string) (bool, error) {
	cfg.Stats.incHead()
	_, err := cfg.Store.HeadObject(ctx, protocol.CloseKey(cfg.Prefix, side, sessionID))
	if err == nil {
		return true, nil
	}
	if objectstore.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

func withDefaults(cfg Config) Config {
	cfg.Prefix = protocol.NormalizePrefix(cfg.Prefix)
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 64 * 1024
	}
	if cfg.ChunkSize < protocol.MinChunkSize {
		cfg.ChunkSize = protocol.MinChunkSize
	}
	if cfg.ChunkSize > protocol.MaxChunkSize {
		cfg.ChunkSize = protocol.MaxChunkSize
	}
	if cfg.FlushDelay < 0 {
		cfg.FlushDelay = 0
	}
	if cfg.PollMin <= 0 {
		cfg.PollMin = 50 * time.Millisecond
	}
	if cfg.PollMax <= 0 {
		cfg.PollMax = 2 * time.Second
	}
	if cfg.PollMax < cfg.PollMin {
		cfg.PollMax = cfg.PollMin
	}
	if cfg.WindowChunks == 0 {
		cfg.WindowChunks = 16
	}
	if cfg.CloseCheckAfterMisses <= 0 {
		cfg.CloseCheckAfterMisses = defaultCloseCheckAfterMisses
	}
	if cfg.ActivePollDuration < 0 {
		cfg.ActivePollDuration = 0
	}
	if cfg.ActivePollDuration == 0 {
		cfg.ActivePollDuration = defaultActivePollDuration
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 2 * time.Minute
	}
	return cfg
}

func nextDelay(current, maxDelay time.Duration) time.Duration {
	if current <= 0 {
		return maxDelay
	}
	next := current * 2
	if next > maxDelay {
		return maxDelay
	}
	return next
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

func closeWrite(v any) {
	type closeWriter interface{ CloseWrite() error }
	if cw, ok := v.(closeWriter); ok {
		_ = cw.CloseWrite()
	}
}

func isClosedConn(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "use of closed network connection") || strings.Contains(s, "closed pipe")
}
