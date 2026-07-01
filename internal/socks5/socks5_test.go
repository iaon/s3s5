package socks5

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"s3s5/internal/protocol"
)

func TestReadRequestAddressTypes(t *testing.T) {
	tests := []struct {
		name string
		req  []byte
		want protocol.Target
	}{
		{
			name: "ipv4",
			req:  append([]byte{Version5, CmdConnect, 0, AtypIPv4, 192, 0, 2, 1}, portBytes(8080)...),
			want: protocol.Target{Type: protocol.AddressIPv4, Host: "192.0.2.1", Port: 8080},
		},
		{
			name: "domain",
			req:  append(append([]byte{Version5, CmdConnect, 0, AtypDomain, 11}, []byte("example.com")...), portBytes(443)...),
			want: protocol.Target{Type: protocol.AddressDomain, Host: "example.com", Port: 443},
		},
		{
			name: "ipv6",
			req:  append(append([]byte{Version5, CmdConnect, 0, AtypIPv6}, net.ParseIP("2001:db8::1").To16()...), portBytes(22)...),
			want: protocol.Target{Type: protocol.AddressIPv6, Host: "2001:db8::1", Port: 22},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, got, err := readRequest(bytes.NewReader(tt.req))
			if err != nil {
				t.Fatal(err)
			}
			if cmd != CmdConnect || got != tt.want {
				t.Fatalf("cmd=%d target=%#v", cmd, got)
			}
		})
	}
}

func portBytes(port uint16) []byte {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], port)
	return b[:]
}
