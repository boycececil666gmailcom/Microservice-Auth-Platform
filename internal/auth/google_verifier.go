package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type googleClaims struct {
	Email           string `json:"email"`
	EmailVerified   bool   `json:"email_verified"`
	Name            string `json:"name"`
	Picture         string `json:"picture"`
	AuthorizedParty string `json:"azp"`
	jwt.RegisteredClaims
}

type GoogleVerifier struct {
	client   *http.Client
	audience string
	jwksURL  string

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

// NewGoogleVerifier creates a verifier for the expected audience and Google JWKS endpoint.
func NewGoogleVerifier(client *http.Client, audience, jwksURL string) *GoogleVerifier {
	return &GoogleVerifier{client: client, audience: audience, jwksURL: jwksURL}
}

// Verify validates a Google ID token and returns its normalized identity data.
//
// Only RS256 is accepted. The signing key is selected by the token's kid header
// and loaded through Google's JWKS endpoint when it is not cached. Verification
// requires the configured audience, expiry, issued-at time, a recognized Google
// issuer, non-empty subject and email claims, a verified email, and a matching
// authorized-party claim when azp is present. Any signature, key-fetch, or claim
// failure is returned as an error.
func (v *GoogleVerifier) Verify(ctx context.Context, rawToken string) (GoogleUser, error) {
	claims := &googleClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, errors.New("Google ID token is missing kid header")
		}
		return v.signingKey(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !token.Valid {
		return GoogleUser{}, fmt.Errorf("Google ID token verification failed: %w", err)
	}
	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return GoogleUser{}, errors.New("Google ID token has an invalid issuer")
	}
	if claims.Subject == "" || claims.Email == "" {
		return GoogleUser{}, errors.New("Google ID token is missing required sub or email claims")
	}
	if !claims.EmailVerified {
		return GoogleUser{}, errors.New("Google ID token email is not verified")
	}
	if claims.AuthorizedParty != "" && claims.AuthorizedParty != v.audience {
		return GoogleUser{}, errors.New("Google ID token has an invalid authorized party")
	}
	name := claims.Name
	if name == "" {
		name = strings.Split(claims.Email, "@")[0]
	}
	return GoogleUser{
		Email: claims.Email, GoogleSub: claims.Subject, Name: name, Picture: claims.Picture,
	}, nil
}

// signingKey returns the RSA signing key identified by kid.
//
// A cached key is used only while the complete JWKS cache is unexpired. On a
// miss or expiry, the method refreshes all keys while holding v.mu so concurrent
// verification cannot trigger duplicate refreshes. It returns an error if the
// endpoint cannot be refreshed or the requested key is absent afterward.
func (v *GoogleVerifier) signingKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if key := v.keys[kid]; key != nil && time.Now().Before(v.expires) {
		return key, nil
	}
	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}
	key := v.keys[kid]
	if key == nil {
		return nil, fmt.Errorf("Google JWKS does not contain key %q", kid)
	}
	return key, nil
}

// refreshKeys replaces the verifier's cached signing keys from the JWKS endpoint.
//
// The response must be HTTP 200, and JSON decoding is limited to the first
// 1 MiB. Only RSA RS256 signing keys with an identifier, an odd exponent of at
// least three, and a modulus of at least 2048 bits are retained. At least one
// usable key is required. On success, the cache lifetime comes from
// Cache-Control max-age, capped at 24 hours, or defaults to one hour.
//
// The caller must hold v.mu while invoking refreshKeys.
func (v *GoogleVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	response, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("Google JWKS endpoint returned %s", response.Status)
	}
	var document struct {
		Keys []struct {
			Kty string `json:"kty"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode Google JWKS: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, item := range document.Keys {
		if item.Kty != "RSA" || item.Alg != "RS256" || item.Kid == "" || (item.Use != "" && item.Use != "sig") {
			continue
		}
		modulus, err := base64.RawURLEncoding.DecodeString(item.N)
		if err != nil || len(modulus) == 0 {
			continue
		}
		exponentBytes, err := base64.RawURLEncoding.DecodeString(item.E)
		if err != nil || len(exponentBytes) == 0 || len(exponentBytes) > 4 {
			continue
		}
		exponent := 0
		for _, value := range exponentBytes {
			exponent = exponent<<8 | int(value)
		}
		if exponent < 3 || exponent%2 == 0 {
			continue
		}
		publicKey := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: exponent}
		if publicKey.N.BitLen() < 2048 {
			continue
		}
		keys[item.Kid] = publicKey
	}
	if len(keys) == 0 {
		return errors.New("Google JWKS endpoint returned no usable RSA keys")
	}
	v.keys = keys
	v.expires = time.Now().Add(jwksCacheDuration(response.Header.Get("Cache-Control")))
	return nil
}

// jwksCacheDuration reads max-age from Cache-Control and caps the cache lifetime at 24 hours.
func jwksCacheDuration(cacheControl string) time.Duration {
	for _, directive := range strings.Split(cacheControl, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(directive), "=")
		if !ok || strings.ToLower(name) != "max-age" {
			continue
		}
		seconds, err := strconv.Atoi(strings.Trim(value, `"`))
		if err == nil && seconds > 0 {
			duration := time.Duration(seconds) * time.Second
			if duration > 24*time.Hour {
				return 24 * time.Hour
			}
			return duration
		}
	}
	return time.Hour
}
