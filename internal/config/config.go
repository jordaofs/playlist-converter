package config

import (
	"encoding/base64"
	"errors"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Address, BaseURL, DatabasePath, AssetsPath, KeyID  string
	Key                                                []byte
	SpotifyID, SpotifySecret, YouTubeID, YouTubeSecret string
	Production                                         bool
	TrustedProxies                                     []netip.Prefix
}

func Load(get func(string) string) (Config, error) {
	value := func(k, d string) string {
		if v := get(k); v != "" {
			return v
		}
		return d
	}
	c := Config{Address: value("LISTEN_ADDR", ":8080"), BaseURL: value("PUBLIC_BASE_URL", "http://localhost:8080"), DatabasePath: value("DATABASE_PATH", "data/converter.db"), AssetsPath: value("ASSETS_PATH", "web/dist"), KeyID: value("TOKEN_KEY_ID", "v1"), SpotifyID: get("SPOTIFY_CLIENT_ID"), SpotifySecret: get("SPOTIFY_CLIENT_SECRET"), YouTubeID: get("YOUTUBE_CLIENT_ID"), YouTubeSecret: get("YOUTUBE_CLIENT_SECRET"), Production: get("APP_ENV") == "production"}
	if c.Production && get("PUBLIC_BASE_URL") == "" {
		return Config{}, errors.New("PUBLIC_BASE_URL must be explicitly configured in production")
	}
	if get("LISTEN_ADDR") == "" && get("PORT") != "" {
		port, err := strconv.Atoi(get("PORT"))
		if err != nil || port < 1 || port > 65535 {
			return Config{}, errors.New("PORT must be an integer between 1 and 65535")
		}
		c.Address = ":" + strconv.Itoa(port)
	}
	var err error
	if c.TrustedProxies, err = trustedProxies(get("TRUSTED_PROXY_CIDRS")); err != nil {
		return Config{}, err
	}
	c.Key, err = base64.StdEncoding.DecodeString(get("TOKEN_ENCRYPTION_KEY"))
	if err != nil || len(c.Key) != 32 {
		return Config{}, errors.New("TOKEN_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return Config{}, errors.New("PUBLIC_BASE_URL must be an origin without credentials, query, or path")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && (u.Scheme != "http" || !local || c.Production) {
		return Config{}, errors.New("PUBLIC_BASE_URL requires HTTPS except local development")
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	c.AssetsPath, err = filepath.Abs(c.AssetsPath)
	if err != nil {
		return Config{}, err
	}
	return c, nil
}

// trustedProxies parses the peers whose X-Forwarded-For header is believed.
// An empty value trusts nobody, so the direct peer stays the client address.
func trustedProxies(value string) ([]netip.Prefix, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	invalid := errors.New("TRUSTED_PROXY_CIDRS must be a comma-separated list of CIDR ranges or addresses")
	var prefixes []netip.Prefix
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, "/") {
			addr, err := netip.ParseAddr(part)
			if err != nil {
				return nil, invalid
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil || prefix != prefix.Masked() {
			return nil, invalid
		}
		// Peers are compared unmapped, so IPv4-mapped ranges become IPv4.
		if addr := prefix.Addr(); addr.Is4In6() {
			if prefix.Bits() < 96 {
				return nil, invalid
			}
			prefix = netip.PrefixFrom(addr.Unmap(), prefix.Bits()-96)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
