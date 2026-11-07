package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	valid := map[string]string{"PUBLIC_BASE_URL": "http://localhost:8080", "TOKEN_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32))}
	for _, tc := range []struct {
		name    string
		changes map[string]string
		ok      bool
	}{
		{"valid", nil, true}, {"missing key", map[string]string{"TOKEN_ENCRYPTION_KEY": ""}, false}, {"invalid key", map[string]string{"TOKEN_ENCRYPTION_KEY": "bad"}, false}, {"https production", map[string]string{"APP_ENV": "production", "PUBLIC_BASE_URL": "https://example.com"}, true}, {"unsafe production", map[string]string{"APP_ENV": "production"}, false}, {"credentials url", map[string]string{"PUBLIC_BASE_URL": "https://a:b@example.com"}, false}, {"path url", map[string]string{"PUBLIC_BASE_URL": "http://localhost:8080/evil"}, false}, {"remote cleartext", map[string]string{"PUBLIC_BASE_URL": "http://example.com"}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			m := map[string]string{}
			for k, v := range valid {
				m[k] = v
			}
			for k, v := range tc.changes {
				m[k] = v
			}
			c, e := Load(func(k string) string { return m[k] })
			if (e == nil) != tc.ok {
				t.Fatalf("got %v", e)
			}
			if tc.ok && (len(c.Key) != 32 || c.DatabasePath == "" || c.Address == "") {
				t.Fatal("defaults missing")
			}
		})
	}
}

func TestHostedConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, port, address, origin, environment, want string
		fail                                           bool
	}{
		{name: "host port", port: "10000", want: ":10000"},
		{name: "minimum", port: "1", want: ":1"},
		{name: "maximum", port: "65535", want: ":65535"},
		{name: "zero", port: "0", fail: true},
		{name: "too large", port: "65536", fail: true},
		{name: "non numeric", port: "http", fail: true},
		{name: "explicit precedence", port: "invalid", address: "127.0.0.1:9090", want: "127.0.0.1:9090"},
		{name: "production requires explicit origin", environment: "production", fail: true},
		{name: "production origin", environment: "production", origin: "https://playlist-converter.example", want: ":8080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{"PORT": tc.port, "LISTEN_ADDR": tc.address, "PUBLIC_BASE_URL": tc.origin, "APP_ENV": tc.environment, "TOKEN_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32))}
			c, e := Load(func(k string) string { return values[k] })
			if (e != nil) != tc.fail {
				t.Fatalf("unexpected error %v", e)
			}
			if !tc.fail && c.Address != tc.want {
				t.Fatalf("address %q want %q", c.Address, tc.want)
			}
		})
	}
}

func TestTrustedProxies(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        []string
		fail        bool
	}{
		{name: "empty keeps direct peer", value: ""},
		{name: "single", value: "172.16.0.0/12", want: []string{"172.16.0.0/12"}},
		{name: "list with spaces and ipv6", value: " 10.0.0.0/8, fd00::/8 ", want: []string{"10.0.0.0/8", "fd00::/8"}},
		{name: "bare address", value: "127.0.0.1", want: []string{"127.0.0.1/32"}},
		{name: "invalid", value: "not-a-cidr", fail: true},
		{name: "empty element", value: "10.0.0.0/8,,", fail: true},
		{name: "host bits", value: "10.0.0.1/8", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{"TRUSTED_PROXY_CIDRS": tc.value, "TOKEN_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32))}
			c, e := Load(func(k string) string { return values[k] })
			if (e != nil) != tc.fail {
				t.Fatalf("unexpected error %v", e)
			}
			if tc.fail {
				if strings.Contains(e.Error(), tc.value) && tc.value != "" {
					t.Fatal("error echoes configured value")
				}
				return
			}
			if len(c.TrustedProxies) != len(tc.want) {
				t.Fatalf("got %v want %v", c.TrustedProxies, tc.want)
			}
			for i, p := range c.TrustedProxies {
				if p.String() != tc.want[i] {
					t.Fatalf("got %v want %v", p, tc.want[i])
				}
			}
		})
	}
}

func TestTrustedProxiesUnmapIPv4(t *testing.T) {
	values := map[string]string{"TRUSTED_PROXY_CIDRS": "::ffff:172.30.53.1/128", "TOKEN_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32))}
	c, e := Load(func(k string) string { return values[k] })
	if e != nil || len(c.TrustedProxies) != 1 || c.TrustedProxies[0].String() != "172.30.53.1/32" {
		t.Fatalf("mapped CIDR not normalized: %v %v", c.TrustedProxies, e)
	}
}
