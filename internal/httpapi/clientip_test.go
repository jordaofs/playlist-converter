package httpapi

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("fd00::/8")}
	for _, tc := range []struct {
		name, remote, forwarded string
		trusted                 []netip.Prefix
		want                    string
	}{
		{name: "direct peer without trust", remote: "203.0.113.7:5000", forwarded: "198.51.100.1", want: "203.0.113.7"},
		{name: "untrusted peer forging header", remote: "203.0.113.7:5000", forwarded: "198.51.100.1", trusted: trusted, want: "203.0.113.7"},
		{name: "trusted proxy single value", remote: "172.18.0.1:5000", forwarded: "198.51.100.1", trusted: trusted, want: "198.51.100.1"},
		{name: "trusted proxy uses rightmost value", remote: "172.18.0.1:5000", forwarded: "10.9.9.9, 198.51.100.2", trusted: trusted, want: "198.51.100.2"},
		{name: "trusted proxy without header", remote: "172.18.0.1:5000", trusted: trusted, want: "172.18.0.1"},
		{name: "trusted proxy with invalid header", remote: "172.18.0.1:5000", forwarded: "not-an-ip", trusted: trusted, want: "172.18.0.1"},
		{name: "trusted ipv6 proxy", remote: "[fd00::1]:5000", forwarded: "2001:db8::5", trusted: trusted, want: "2001:db8::5"},
		{name: "ipv4 mapped peer", remote: "[::ffff:172.18.0.1]:5000", forwarded: "198.51.100.3", trusted: trusted, want: "198.51.100.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remote
			if tc.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			if got := clientIP(r, tc.trusted); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

// An IPv6 client controls at least a /64, so per-address buckets would let a
// single client rotate addresses past its limit and fill the bucket table.
func TestRateKeyGroupsIPv6Networks(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"198.51.100.7", "198.51.100.7"},
		{"2001:db8:1:2:aaaa::1", "2001:db8:1:2::/64"},
		{"2001:db8:1:2:ffff:ffff:ffff:ffff", "2001:db8:1:2::/64"},
		{"2001:db8:1:3::1", "2001:db8:1:3::/64"},
		{"not-an-ip", "not-an-ip"},
	} {
		if got := rateKey(tc.in); got != tc.want {
			t.Fatalf("rateKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRateKeyUnmapsIPv4(t *testing.T) {
	if a, b := rateKey("::ffff:198.51.100.7"), rateKey("::ffff:198.51.100.8"); a == b || a != "198.51.100.7" {
		t.Fatalf("IPv4-mapped clients share a bucket: %q %q", a, b)
	}
}
