package tunnel

import (
	"context"
	"io"
	"net"
	"time"
)

type FlushReason string

const (
	FlushSize     FlushReason = "size"
	FlushDeadline FlushReason = "deadline"
	FlushEOF      FlushReason = "eof"
	FlushError    FlushReason = "error"
)

type AggregatedRead struct {
	Data        []byte
	Reason      FlushReason
	SocketReads int
}

type Aggregator struct {
	MaxBytes   int
	FlushDelay time.Duration
}

func (a Aggregator) Read(ctx context.Context, r io.Reader) (AggregatedRead, error) {
	maxBytes := a.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	buf := make([]byte, maxBytes)
	n, err := r.Read(buf)
	if n <= 0 {
		return AggregatedRead{}, err
	}
	out := append([]byte(nil), buf[:n]...)
	if err != nil {
		return AggregatedRead{Data: out, Reason: reasonForErr(err), SocketReads: 1}, err
	}
	if len(out) >= maxBytes {
		return AggregatedRead{Data: out, Reason: FlushSize, SocketReads: 1}, nil
	}
	if a.FlushDelay == 0 || !setReadDeadline(r, time.Now().Add(a.FlushDelay)) {
		return AggregatedRead{Data: out, Reason: FlushDeadline, SocketReads: 1}, nil
	}
	defer clearReadDeadline(r)
	socketReads := 1
	for len(out) < maxBytes {
		if err := ctx.Err(); err != nil {
			return AggregatedRead{Data: out, Reason: FlushError}, err
		}
		n, err = r.Read(buf[:maxBytes-len(out)])
		socketReads++
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if len(out) >= maxBytes {
			return AggregatedRead{Data: out, Reason: FlushSize, SocketReads: socketReads}, err
		}
		if err != nil {
			if isTimeout(err) {
				return AggregatedRead{Data: out, Reason: FlushDeadline, SocketReads: socketReads}, nil
			}
			return AggregatedRead{Data: out, Reason: reasonForErr(err), SocketReads: socketReads}, err
		}
	}
	return AggregatedRead{Data: out, Reason: FlushSize, SocketReads: socketReads}, nil
}

func reasonForErr(err error) FlushReason {
	if err == io.EOF {
		return FlushEOF
	}
	return FlushError
}

func setReadDeadline(r io.Reader, at time.Time) bool {
	type readDeadliner interface{ SetReadDeadline(time.Time) error }
	if d, ok := r.(readDeadliner); ok {
		return d.SetReadDeadline(at) == nil
	}
	return false
}

func clearReadDeadline(r io.Reader) {
	type readDeadliner interface{ SetReadDeadline(time.Time) error }
	if d, ok := r.(readDeadliner); ok {
		_ = d.SetReadDeadline(time.Time{})
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	return false
}
