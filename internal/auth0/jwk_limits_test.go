package auth0

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-jose/go-jose/v4"
)

func jwksServer(t *testing.T, bits int) JWKClientOptions {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"keys": []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k", Algorithm: "RS256", Use: "sig"}}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	return JWKClientOptions{URI: srv.URL}
}

// A JWKS endpoint cannot make the gateway accept an RSA key above 4096 bits: go-jose v4.1.6 caps the modulus, which
// keeps a hostile or broken identity provider from making every verification expensive.
func TestJWKClient_refusesOversizedRSAKeys(t *testing.T) {
	keys, err := NewJWKClient(jwksServer(t, 2048), nil).downloadKeys()
	if err != nil || len(keys) != 1 {
		t.Fatalf("a 2048 bit key must be accepted: %d keys, %v", len(keys), err)
	}
	if keys, err := NewJWKClient(jwksServer(t, 4104), nil).downloadKeys(); err == nil && len(keys) > 0 {
		t.Errorf("a 4104 bit RSA key must be refused, got %d keys", len(keys))
	}
}
