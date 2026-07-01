package policy

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"strings"
	"testing"

	"s3s5/internal/protocol"
)

func TestUnsafeAddressesDeniedByDefault(t *testing.T) {
	engine, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	unsafe := []string{
		"0.0.0.0",
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.1.1",
		"169.254.169.254",
		"::",
		"::1",
		"fc00::1",
		"fe80::1",
		"ff02::1",
		"fd00:ec2::254",
	}
	for _, raw := range unsafe {
		if err := engine.CheckAddr(netip.MustParseAddr(raw)); err == nil {
			t.Fatalf("expected %s to be denied", raw)
		}
	}
	if err := engine.CheckAddr(netip.MustParseAddr("2001:4860:4860::8888")); err != nil {
		t.Fatalf("public IPv6 should be allowed: %v", err)
	}
}

func TestAllowPrivateOverridesUnsafeAddressDenial(t *testing.T) {
	engine, err := New(Config{AllowPrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.CheckAddr(netip.MustParseAddr("127.0.0.1")); err != nil {
		t.Fatalf("allow private should allow loopback for local tests: %v", err)
	}
}

func TestRuleParsing(t *testing.T) {
	r, err := parseRule("*.example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	if !r.hasPort || r.port != 443 || r.suffix != ".example.com" {
		t.Fatalf("unexpected suffix rule: %#v", r)
	}
	r, err = parseRule("2001:db8::/32")
	if err != nil {
		t.Fatal(err)
	}
	if r.cidr == nil || !r.cidr.Contains(netip.MustParseAddr("2001:db8::1")) {
		t.Fatalf("unexpected CIDR rule: %#v", r)
	}
}

func TestCIDRRulesApplyToResolvedDomainIPs(t *testing.T) {
	resolver, stop := testResolver(t, map[string]netip.Addr{
		"example.test": netip.MustParseAddr("203.0.113.10"),
	})
	defer stop()

	denyEngine, err := New(Config{DenyTargets: []string{"203.0.113.0/24"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := denyEngine.CheckTarget(context.Background(), testTarget("example.test"), resolver); err == nil {
		t.Fatal("expected resolved domain IP to be denied by CIDR rule")
	}

	allowEngine, err := New(Config{AllowTargets: []string{"203.0.113.0/24"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := allowEngine.CheckTarget(context.Background(), testTarget("example.test"), resolver); err != nil {
		t.Fatalf("expected resolved domain IP to satisfy CIDR allow rule: %v", err)
	}
}

func TestRequireExplicitAllowlistOrUnrestrictedOptIn(t *testing.T) {
	if _, err := New(Config{RequireAllowTarget: true}); err == nil {
		t.Fatal("expected explicit allowlist or unrestricted opt-in to be required")
	}
	if _, err := New(Config{RequireAllowTarget: true, AllowUnrestricted: true}); err != nil {
		t.Fatalf("allow-unrestricted opt-in should be accepted: %v", err)
	}
	if _, err := New(Config{RequireAllowTarget: true, AllowTargets: []string{"203.0.113.0/24"}}); err != nil {
		t.Fatalf("explicit allowlist should be accepted: %v", err)
	}
}

func testTarget(host string) protocol.Target {
	return protocol.Target{Type: protocol.AddressDomain, Host: host, Port: 443}
}

func testResolver(t *testing.T, records map[string]netip.Addr) (*net.Resolver, func()) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			resp, ok := dnsResponse(buf[:n], records)
			if !ok {
				continue
			}
			_, _ = pc.WriteTo(resp, addr)
		}
	}()
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", pc.LocalAddr().String())
		},
	}
	return resolver, func() {
		_ = pc.Close()
		<-done
	}
}

func dnsResponse(query []byte, records map[string]netip.Addr) ([]byte, bool) {
	if len(query) < 12 {
		return nil, false
	}
	qname, qtype, qend, ok := dnsQuestion(query)
	if !ok {
		return nil, false
	}
	addr, ok := records[strings.TrimSuffix(strings.ToLower(qname), ".")]
	ancount := uint16(0)
	answer := make([]byte, 0, 32)
	if ok {
		switch qtype {
		case 1:
			if addr.Is4() {
				ancount = 1
				a4 := addr.As4()
				answer = appendDNSAnswer(answer, 1, a4[:])
			}
		case 28:
			if addr.Is6() {
				ancount = 1
				a16 := addr.As16()
				answer = appendDNSAnswer(answer, 28, a16[:])
			}
		}
	}
	resp := make([]byte, 0, len(query)+len(answer))
	resp = append(resp, query[:2]...)
	resp = append(resp, 0x81, 0x80)
	var counts [8]byte
	binary.BigEndian.PutUint16(counts[0:2], 1)
	binary.BigEndian.PutUint16(counts[2:4], ancount)
	resp = append(resp, counts[:]...)
	resp = append(resp, query[12:qend+4]...)
	resp = append(resp, answer...)
	return resp, true
}

func dnsQuestion(query []byte) (string, uint16, int, bool) {
	if len(query) < 18 {
		return "", 0, 0, false
	}
	i := 12
	var parts []string
	for {
		if i >= len(query) {
			return "", 0, 0, false
		}
		n := int(query[i])
		i++
		if n == 0 {
			break
		}
		if i+n > len(query) {
			return "", 0, 0, false
		}
		parts = append(parts, string(query[i:i+n]))
		i += n
	}
	if i+4 > len(query) {
		return "", 0, 0, false
	}
	return strings.Join(parts, "."), binary.BigEndian.Uint16(query[i : i+2]), i, true
}

func appendDNSAnswer(dst []byte, typ uint16, addr []byte) []byte {
	dst = append(dst, 0xc0, 0x0c)
	var header [10]byte
	binary.BigEndian.PutUint16(header[0:2], typ)
	binary.BigEndian.PutUint16(header[2:4], 1)
	binary.BigEndian.PutUint32(header[4:8], 0)
	binary.BigEndian.PutUint16(header[8:10], uint16(len(addr)))
	dst = append(dst, header[:]...)
	dst = append(dst, addr...)
	return dst
}
