package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// clientIP returns the address used for per-client limits. X-Forwarded-For is
// only believed when the direct peer is a configured proxy, and then only its
// rightmost entry: that is the value the proxy itself appended, while anything
// to the left may have been supplied by the client.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !contains(trusted, peer.Unmap()) {
		return host
	}
	values := r.Header.Values("X-Forwarded-For")
	if len(values) == 0 {
		return peer.Unmap().String()
	}
	entries := strings.Split(values[len(values)-1], ",")
	forwarded, err := netip.ParseAddr(strings.TrimSpace(entries[len(entries)-1]))
	if err != nil {
		return peer.Unmap().String()
	}
	return forwarded.Unmap().String()
}

func contains(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// rateKey is the rate-limit bucket for a client address. IPv6 clients are
// grouped by /64 because a single subscriber can rotate freely inside it.
func rateKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	if addr = addr.Unmap(); addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr, 64).Masked().String()
}
