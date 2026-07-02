package socks5

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"s3s5/internal/protocol"
)

const (
	Version5 = 0x05

	MethodNoAuth       = 0x00
	MethodNoAcceptable = 0xff

	CmdConnect      = 0x01
	CmdBind         = 0x02
	CmdUDPAssociate = 0x03

	AtypIPv4   = 0x01
	AtypDomain = 0x03
	AtypIPv6   = 0x04

	ReplySucceeded              = 0x00
	ReplyGeneralFailure         = 0x01
	ReplyConnectionNotAllowed   = 0x02
	ReplyNetworkUnreachable     = 0x03
	ReplyHostUnreachable        = 0x04
	ReplyConnectionRefused      = 0x05
	ReplyTTLExpired             = 0x06
	ReplyCommandNotSupported    = 0x07
	ReplyAddressTypeUnsupported = 0x08
)

type ConnectHandler func(ctx context.Context, target protocol.Target, conn net.Conn, reply func(byte) error) error

type Server struct {
	Handler ConnectHandler
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Time{})
	}
	if err := negotiate(conn); err != nil {
		return
	}
	cmd, target, err := readRequest(conn)
	if err != nil {
		_ = writeReply(conn, ReplyGeneralFailure)
		return
	}
	if cmd != CmdConnect {
		_ = writeReply(conn, ReplyCommandNotSupported)
		return
	}
	if s.Handler == nil {
		_ = writeReply(conn, ReplyGeneralFailure)
		return
	}
	replied := false
	reply := func(code byte) error {
		if replied {
			return nil
		}
		replied = true
		return writeReply(conn, code)
	}
	if err := s.Handler(ctx, target, conn, reply); err != nil && !replied {
		_ = reply(ReplyGeneralFailure)
	}
}

func negotiate(rw io.ReadWriter) error {
	head := make([]byte, 2)
	if _, err := io.ReadFull(rw, head); err != nil {
		return err
	}
	if head[0] != Version5 {
		return fmt.Errorf("unsupported SOCKS version")
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(rw, methods); err != nil {
		return err
	}
	for _, m := range methods {
		if m == MethodNoAuth {
			_, err := rw.Write([]byte{Version5, MethodNoAuth})
			return err
		}
	}
	_, _ = rw.Write([]byte{Version5, MethodNoAcceptable})
	return fmt.Errorf("no acceptable SOCKS5 auth method")
}

func readRequest(r io.Reader) (byte, protocol.Target, error) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(r, head); err != nil {
		return 0, protocol.Target{}, err
	}
	if head[0] != Version5 || head[2] != 0x00 {
		return 0, protocol.Target{}, fmt.Errorf("invalid SOCKS5 request")
	}
	cmd, atyp := head[1], head[3]
	var target protocol.Target
	switch atyp {
	case AtypIPv4:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, target, err
		}
		target.Type = protocol.AddressIPv4
		target.Host = net.IP(buf).String()
	case AtypIPv6:
		buf := make([]byte, 16)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, target, err
		}
		target.Type = protocol.AddressIPv6
		target.Host = net.IP(buf).String()
	case AtypDomain:
		var l [1]byte
		if _, err := io.ReadFull(r, l[:]); err != nil {
			return 0, target, err
		}
		buf := make([]byte, int(l[0]))
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, target, err
		}
		target.Type = protocol.AddressDomain
		target.Host = string(buf)
	default:
		return cmd, target, fmt.Errorf("unsupported address type")
	}
	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return 0, target, err
	}
	target.Port = binary.BigEndian.Uint16(port[:])
	return cmd, target, nil
}

func writeReply(w io.Writer, code byte) error {
	_, err := w.Write([]byte{Version5, code, 0x00, AtypIPv4, 0, 0, 0, 0, 0, 0})
	return err
}
