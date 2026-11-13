package auth0

import (
	"net/http"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// validate parses a token and checks its claims with the key, the way the request path does after it has the secret.
func validate(audience []string, key []byte, token string) error {
	v := NewValidator(NewConfiguration(NewKeyProvider(key), audience, defaultIssuer, jose.HS256), nil)
	parsed, err := jwt.ParseSigned(token, SignatureAlgorithms)
	if err != nil {
		return err
	}
	_, err = v.ValidateTokenClaims(parsed, key)

	return err
}

// Every configured audience must be in the token (what go-jose v3 enforced; v4 alone would accept any one of them).
func TestValidator_everyConfiguredAudienceIsRequired(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	for name, tc := range map[string]struct {
		tokenAudience []string
		ok            bool
	}{
		"both":           {[]string{"api", "admin"}, true},
		"both and more":  {[]string{"other", "admin", "api"}, true},
		"only one":       {[]string{"api"}, false},
		"only the other": {[]string{"admin"}, false},
		"none of them":   {[]string{"other"}, false},
		"no audience":    {nil, false},
	} {
		err := validate([]string{"api", "admin"}, defaultSecret, getTestToken(tc.tokenAudience, defaultIssuer, exp, jose.HS256, defaultSecret))
		if (err == nil) != tc.ok {
			t.Errorf("%s: error %v, want ok=%v", name, err, tc.ok)
		}
	}
}

// With no audience configured the audience of the token is not checked.
func TestValidator_noConfiguredAudience(t *testing.T) {
	if err := validate(nil, defaultSecret, getTestToken([]string{"whatever"}, defaultIssuer, time.Now().Add(time.Hour), jose.HS256, defaultSecret)); err != nil {
		t.Error(err)
	}
}

// A symmetric key shorter than the hash output is refused (RFC 7518 3.2), for the secret the gateway holds.
func TestValidator_shortHMACKeyIsRefused(t *testing.T) {
	token := getTestToken(defaultAudience, defaultIssuer, time.Now().Add(time.Hour), jose.HS256, defaultSecret)
	if err := validate(defaultAudience, defaultSecret, token); err != nil {
		t.Fatalf("a 32 byte key must work: %v", err)
	}
	if err := validate(defaultAudience, defaultSecret[:16], token); err == nil {
		t.Error("a 16 byte HS256 key must be refused")
	}
}

// A token that says it is not signed, or uses an algorithm the gateway does not support, never gets past parsing.
func TestExtraction_refusesUnsupportedAlgorithms(t *testing.T) {
	for name, token := range map[string]string{
		"none":    "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJpc3MiOiJpc3N1ZXIifQ.",
		"unknown": "eyJhbGciOiJGT08iLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJpc3N1ZXIifQ.c2ln",
	} {
		r, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
		r.Header.Set("Authorization", "Bearer "+token)
		if _, err := FromHeader(r); err == nil {
			t.Errorf("%s: the token must be refused", name)
		}
	}
}
