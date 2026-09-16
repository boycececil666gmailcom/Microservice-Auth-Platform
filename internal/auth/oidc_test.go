package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestBuildGoogleAuthURL(t *testing.T) {
	result, err := BuildGoogleAuthURL("client-id", "http://localhost/callback", "custom_state_456")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{googleAuthEndpoint, "client_id=client-id", "response_type=code", "state=custom_state_456"} {
		if !strings.Contains(result, expected) {
			t.Errorf("URL %q does not contain %q", result, expected)
		}
	}
}

func TestGenerateStateTokenIsUnique(t *testing.T) {
	first, err := GenerateStateToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateStateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 10 || first == second {
		t.Fatal("state tokens must be non-empty and unique")
	}
}

func TestMockGoogleIDTokenRoundTrip(t *testing.T) {
	token, err := BuildMockGoogleIDToken("mock-client-id", "alice@gmail.com", "google_sub_987654321", "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}
	user, err := ParseMockGoogleIDToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "alice@gmail.com" || user.GoogleSub != "google_sub_987654321" || user.Name != "Alice Smith" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestParseMockGoogleIDTokenRejectsMalformed(t *testing.T) {
	if _, err := ParseMockGoogleIDToken("invalid.jwt.string.with.too.many.parts"); err == nil {
		t.Fatal("malformed token was accepted")
	}
}

func TestGoogleVerifierValidatesSignatureAndClaims(t *testing.T) {
	key := testPrivateKey(t)
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		_ = json.NewEncoder(w).Encode(PublicJWKS(&key.PublicKey))
	}))
	defer jwksServer.Close()

	issuedAt := time.Now().Add(-time.Minute)
	rawToken := signedGoogleToken(t, key, "client-id", true, issuedAt)
	verifier := NewGoogleVerifier(jwksServer.Client(), "client-id", jwksServer.URL)
	user, err := verifier.Verify(context.Background(), rawToken)
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "alice@gmail.com" || user.GoogleSub != "google-sub" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestGoogleVerifierRejectsWrongAudienceAndUnverifiedEmail(t *testing.T) {
	key := testPrivateKey(t)
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(PublicJWKS(&key.PublicKey))
	}))
	defer jwksServer.Close()
	verifier := NewGoogleVerifier(jwksServer.Client(), "client-id", jwksServer.URL)

	if _, err := verifier.Verify(context.Background(), signedGoogleToken(t, key, "wrong-client", true, time.Now())); err == nil {
		t.Fatal("token with wrong audience was accepted")
	}
	if _, err := verifier.Verify(context.Background(), signedGoogleToken(t, key, "client-id", false, time.Now())); err == nil {
		t.Fatal("token with unverified email was accepted")
	}
}

func signedGoogleToken(t *testing.T, key *rsa.PrivateKey, audience string, verified bool, issuedAt time.Time) string {
	t.Helper()
	claims := googleClaims{
		Email: "alice@gmail.com", EmailVerified: verified, Name: "Alice",
		AuthorizedParty: audience,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "https://accounts.google.com", Subject: "google-sub",
			Audience: jwt.ClaimStrings{audience}, IssuedAt: jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = JWTKeyID
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
