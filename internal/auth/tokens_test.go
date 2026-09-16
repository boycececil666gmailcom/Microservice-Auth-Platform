package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessTokenRS256ClaimsAndKeyID(t *testing.T) {
	key := testPrivateKey(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	tokenString, err := CreateAccessToken(key, 15*time.Minute, "alice@example.com", "google_oidc", now)
	if err != nil {
		t.Fatal(err)
	}
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || !token.Valid {
		t.Fatalf("token validation failed: %v", err)
	}
	if token.Header["kid"] != JWTKeyID {
		t.Fatalf("kid = %v, want %s", token.Header["kid"], JWTKeyID)
	}
	if claims.Subject != "alice@example.com" || claims.Email != "alice@example.com" || claims.SSOProvider != "google_oidc" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestRefreshTokensAreRandomAndStoredByDigest(t *testing.T) {
	first, err := GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 || first == second {
		t.Fatalf("unexpected tokens: lengths %d and %d", len(first), len(second))
	}
	key := refreshTokenKey(first)
	if key == "refresh_token:"+first || len(key) <= len("refresh_token:") {
		t.Fatalf("refresh token key exposes the bearer token: %q", key)
	}
}

func TestPublicJWKSMatchesSigningKey(t *testing.T) {
	key := testPrivateKey(t)
	document := PublicJWKS(&key.PublicKey)
	keys := document["keys"].([]map[string]string)
	if len(keys) != 1 || keys[0]["kid"] != JWTKeyID || keys[0]["alg"] != "RS256" {
		t.Fatalf("unexpected JWKS: %#v", document)
	}
	modulus, err := base64.RawURLEncoding.DecodeString(keys[0]["n"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(modulus, key.N.Bytes()) {
		t.Fatal("JWKS modulus does not match signing key")
	}
}

func TestParsePrivateKey(t *testing.T) {
	key := testPrivateKey(t)
	pemValue := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(t, key)})
	parsed, err := ParsePrivateKey(string(pemValue))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.N.Cmp(key.N) != 0 {
		t.Fatal("parsed private key does not match")
	}
}

func testPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	value, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
