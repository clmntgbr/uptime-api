package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

var ErrBlocked = errors.New("destination is not allowed")

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::/128",
	"::1/128",
	"64:ff9b::/96",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
)

func mustPrefixes(values ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		out = append(out, netip.MustParsePrefix(v))
	}
	return out
}

// IsBlocked reports whether ip is in a private, loopback, link-local (cloud
// metadata) or otherwise internal range. IPv4-mapped IPv6 addresses are checked as IPv4.
func IsBlocked(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// dialer resolves the host once, checks every resolved address and connects
// to the checked IP directly, so DNS rebinding cannot swap the target. Every
// connection goes through it, including the ones opened by redirects.
type dialer struct {
	allowPrivate bool
	ipVersion    string
	resolver     *net.Resolver
	net          net.Dialer
	onDial       func(host string, ip netip.Addr)
}

func (d *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	var candidates []netip.Addr
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		candidates = []netip.Addr{ip}
	} else {
		addrs, err := d.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		candidates = addrs
	}

	var lastErr error = &net.DNSError{Err: "no address for the requested ip version", Name: host, IsNotFound: true}
	for _, ip := range candidates {
		ip = ip.Unmap()
		if d.ipVersion == "ipv4" && !ip.Is4() || d.ipVersion == "ipv6" && !ip.Is6() {
			continue
		}
		if !d.allowPrivate && IsBlocked(ip) {
			// Do not include the resolved IP: it would help map internal networks.
			lastErr = fmt.Errorf("%w: %s", ErrBlocked, host)
			continue
		}
		dialNetwork := "tcp4"
		if ip.Is6() {
			dialNetwork = "tcp6"
		}
		conn, err := d.net.DialContext(ctx, dialNetwork, net.JoinHostPort(ip.String(), port))
		if err != nil {
			lastErr = err
			continue
		}
		if d.onDial != nil {
			d.onDial(host, ip)
		}
		return conn, nil
	}
	return nil, lastErr
}

func newDialer(allowPrivate bool, ipVersion string, timeout time.Duration, onDial func(string, netip.Addr)) *dialer {
	return &dialer{
		allowPrivate: allowPrivate,
		ipVersion:    ipVersion,
		resolver:     net.DefaultResolver,
		net:          net.Dialer{Timeout: timeout, KeepAlive: -1},
		onDial:       onDial,
	}
}
