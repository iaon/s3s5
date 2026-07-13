package crypto

import (
	"bytes"
	"testing"
)

func TestPSKCodecEncryptsAndAuthenticates(t *testing.T) {
	codec, err := NewPSKCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("target=example.com:443 payload secret")
	sealed, err := codec.Seal("open", "session-a", "control", 0, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("example.com")) || bytes.Contains(sealed, []byte("payload secret")) {
		t.Fatalf("sealed payload leaked plaintext: %s", string(sealed))
	}
	got, err := codec.Open("open", "session-a", "control", 0, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("roundtrip mismatch: %q", got)
	}
	if _, err := codec.Open("open", "session-a", "control", 1, sealed); err == nil {
		t.Fatal("expected AAD sequence mismatch to fail")
	}
	other, err := NewPSKCodec("abcdef0123456789abcdef0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Open("open", "session-a", "control", 0, sealed); err == nil {
		t.Fatal("expected wrong key to fail")
	}
	sealed[len(sealed)-2] ^= 1
	if _, err := codec.Open("open", "session-a", "control", 0, sealed); err == nil {
		t.Fatal("expected tamper to fail")
	}
}

func TestPSKCodecRejectsMalformedEnvelopeWithoutPanic(t *testing.T) {
	codec, err := NewPSKCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"not-json":        []byte("{"),
		"bad-nonce":       []byte(`{"v":1,"alg":"AES-256-GCM","nonce":"%%%","ciphertext":"AA=="}`),
		"bad-version":     []byte(`{"v":2,"alg":"AES-256-GCM","nonce":"AA==","ciphertext":"AA=="}`),
		"bad-ct":          []byte(`{"v":1,"alg":"AES-256-GCM","nonce":"AA==","ciphertext":"%%%"} `),
		"short-plaintext": []byte(`{"v":1,"alg":"AES-256-GCM","nonce":"AA==","ciphertext":"AA=="}`),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Open panicked for malformed envelope: %v", r)
				}
			}()
			if _, err := codec.Open("open", "session-a", "control", 0, data); err == nil {
				t.Fatal("expected malformed envelope to fail")
			}
		})
	}
}

func TestPSKCodecBinaryDataEnvelopeRoundTripAndTamper(t *testing.T) {
	codec, err := NewPSKCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	plain := bytes.Repeat([]byte("x"), 64*1024)
	sealed, err := codec.SealData("session-a", "c2s", 7, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, plain[:64]) {
		t.Fatal("binary data envelope leaked plaintext")
	}
	if bytes.Contains(sealed, []byte("ciphertext")) {
		t.Fatal("binary data envelope should not contain JSON field names")
	}
	got, err := codec.OpenData("session-a", "c2s", 7, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("binary data round trip mismatch")
	}
	if _, err := codec.OpenData("session-a", "s2c", 7, sealed); err == nil {
		t.Fatal("expected direction mismatch to fail")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := codec.OpenData("session-a", "c2s", 7, sealed); err == nil {
		t.Fatal("expected tampered binary data envelope to fail")
	}
}
