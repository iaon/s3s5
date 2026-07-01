package policy

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"s3s5/internal/protocol"
)

type Config struct {
	AllowPrivate       bool
	AllowUnrestricted  bool
	RequireAllowTarget bool
	AllowTargets       []string
	DenyTargets        []string
}

type Engine struct {
	allowPrivate      bool
	allowUnrestricted bool
	allowRules        []rule
	denyRules         []rule
}

type rule struct {
	raw     string
	host    string
	suffix  string
	cidr    *netip.Prefix
	port    uint16
	hasPort bool
	any     bool
}

func New(cfg Config) (*Engine, error) {
	if cfg.RequireAllowTarget && !cfg.AllowUnrestricted && len(cfg.AllowTargets) == 0 {
		return nil, fmt.Errorf("at least one --allow-target is required, or pass --allow-target '*' to allow unrestricted public egress")
	}
	e := &Engine{allowPrivate: cfg.AllowPrivate, allowUnrestricted: cfg.AllowUnrestricted}
	for _, raw := range cfg.AllowTargets {
		r, err := parseRule(raw)
		if err != nil {
			return nil, fmt.Errorf("allow target %q: %w", raw, err)
		}
		e.allowRules = append(e.allowRules, r)
	}
	for _, raw := range cfg.DenyTargets {
		r, err := parseRule(raw)
		if err != nil {
			return nil, fmt.Errorf("deny target %q: %w", raw, err)
		}
		e.denyRules = append(e.denyRules, r)
	}
	return e, nil
}

func (e *Engine) CheckTarget(ctx context.Context, t protocol.Target, resolver *net.Resolver) error {
	_, err := e.allowedIPs(ctx, t, resolver)
	return err
}

func (e *Engine) CheckAddr(ip netip.Addr) error {
	if e.allowPrivate {
		return nil
	}
	if isUnsafe(ip) {
		return fmt.Errorf("unsafe destination address denied: %s", ip)
	}
	return nil
}

func (e *Engine) AllowedIPs(ctx context.Context, t protocol.Target, resolver *net.Resolver) ([]netip.Addr, error) {
	return e.allowedIPs(ctx, t, resolver)
}

func (e *Engine) allowedIPs(ctx context.Context, t protocol.Target, resolver *net.Resolver) ([]netip.Addr, error) {
	if t.Port == 0 {
		return nil, fmt.Errorf("port 0 is not allowed")
	}
	ips, literal, err := e.resolveTarget(ctx, t, resolver)
	if err != nil {
		return nil, err
	}
	if e.matches(e.denyRules, t, literal) || e.matchesAnyIP(e.denyRules, t, ips) {
		return nil, fmt.Errorf("target denied by rule")
	}
	if !e.allowUnrestricted && len(e.allowRules) > 0 && !e.matches(e.allowRules, t, literal) && !e.matchesAnyIP(e.allowRules, t, ips) {
		return nil, fmt.Errorf("target not allowed by allowlist")
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if err := e.CheckAddr(ip); err != nil {
			return nil, err
		}
		out = append(out, ip)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no allowed resolved addresses")
	}
	return out, nil
}

func (e *Engine) matches(rules []rule, t protocol.Target, ip *netip.Addr) bool {
	for _, r := range rules {
		if r.matches(t, ip) {
			return true
		}
	}
	return false
}

func (e *Engine) matchesAnyIP(rules []rule, t protocol.Target, ips []netip.Addr) bool {
	for _, ip := range ips {
		addr := ip
		if e.matches(rules, t, &addr) {
			return true
		}
	}
	return false
}

func (e *Engine) resolveTarget(ctx context.Context, t protocol.Target, resolver *net.Resolver) ([]netip.Addr, *netip.Addr, error) {
	if ip, ok := parseAddr(t.Host); ok {
		return []netip.Addr{ip}, &ip, nil
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	ips, err := resolver.LookupNetIP(ctx, "ip", t.Host)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve target: %w", err)
	}
	if len(ips) == 0 {
		return nil, nil, fmt.Errorf("resolve target: no addresses")
	}
	return ips, nil, nil
}

func (r rule) matches(t protocol.Target, ip *netip.Addr) bool {
	if r.hasPort && r.port != t.Port {
		return false
	}
	if r.any {
		return true
	}
	host := strings.ToLower(strings.Trim(t.Host, "[]"))
	if r.host != "" && host == r.host {
		return true
	}
	if r.suffix != "" && (host == strings.TrimPrefix(r.suffix, ".") || strings.HasSuffix(host, r.suffix)) {
		return true
	}
	if r.cidr != nil {
		addr := ip
		if addr == nil {
			if parsed, ok := parseAddr(t.Host); ok {
				addr = &parsed
			}
		}
		return addr != nil && r.cidr.Contains(*addr)
	}
	return false
}

func parseRule(raw string) (rule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return rule{}, fmt.Errorf("empty rule")
	}
	r := rule{raw: raw}
	hostPart := raw
	portPart := ""
	if strings.HasPrefix(raw, "*:") {
		r.any = true
		portPart = strings.TrimPrefix(raw, "*:")
		hostPart = "*"
	} else if h, p, ok := splitHostPortLoose(raw); ok {
		hostPart, portPart = h, p
	}
	if portPart != "" {
		n, err := strconv.ParseUint(portPart, 10, 16)
		if err != nil || n == 0 {
			return rule{}, fmt.Errorf("invalid port")
		}
		r.port = uint16(n)
		r.hasPort = true
	}
	hostPart = strings.Trim(strings.ToLower(hostPart), "[]")
	if hostPart == "*" {
		r.any = true
		return r, nil
	}
	if pfx, err := netip.ParsePrefix(hostPart); err == nil {
		r.cidr = &pfx
		return r, nil
	}
	if addr, err := netip.ParseAddr(hostPart); err == nil {
		pfx := netip.PrefixFrom(addr, addr.BitLen())
		r.cidr = &pfx
		return r, nil
	}
	if strings.HasPrefix(hostPart, ".") || strings.HasPrefix(hostPart, "*.") {
		r.suffix = "." + strings.TrimPrefix(strings.TrimPrefix(hostPart, "*"), ".")
		return r, nil
	}
	r.host = hostPart
	return r, nil
}

func splitHostPortLoose(s string) (string, string, bool) {
	if strings.HasPrefix(s, "[") {
		h, p, err := net.SplitHostPort(s)
		return h, p, err == nil
	}
	if strings.Count(s, ":") == 1 {
		i := strings.LastIndexByte(s, ':')
		return s[:i], s[i+1:], true
	}
	return s, "", false
}

func parseAddr(host string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return ip, err == nil
}

func isUnsafe(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip == netip.MustParseAddr("169.254.169.254") || ip == netip.MustParseAddr("fd00:ec2::254") {
		return true
	}
	return false
}
