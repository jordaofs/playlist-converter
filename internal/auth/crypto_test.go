package auth

import (
	"bytes"
	"testing"
)

func TestTokenEncryption(t *testing.T) {
	key := bytes.Repeat([]byte{4}, 32)
	plain := []byte("secret-refresh-token")
	v, e := seal(key, "p/spotify/v1", plain)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(v, plain) {
		t.Fatal("plaintext")
	}
	out, e := open(key, "p/spotify/v1", v)
	if e != nil || !bytes.Equal(out, plain) {
		t.Fatal("roundtrip", e)
	}
	if _, e = open(key, "other", v); e == nil {
		t.Fatal("wrong context accepted")
	}
	bad := append([]byte(nil), v...)
	bad[len(bad)-1] ^= 1
	if _, e = open(key, "p/spotify/v1", bad); e == nil {
		t.Fatal("tampering accepted")
	}
	if _, e = open(key, "p", nil); e == nil {
		t.Fatal("short accepted")
	}
	if _, e = seal(nil, "p", plain); e == nil {
		t.Fatal("bad key accepted")
	}
	if _, e = open(nil, "p", v); e == nil {
		t.Fatal("bad key accepted")
	}
}
