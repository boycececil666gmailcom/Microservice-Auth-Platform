package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const JWTKeyID = "auth-key-1"

type accessClaims struct {
	Email       string `json:"email"`
	SSOProvider string `json:"sso_provider"`
	jwt.RegisteredClaims
}

// ParsePrivateKey parses an RSA private key from PKCS#8 PEM text.
//
// It rejects malformed PEM, non-PKCS#8 or non-RSA keys, RSA moduli smaller
// than 2048 bits, and keys that fail rsa.PrivateKey.Validate. The returned key
// is ready for RS256 signing.
func ParsePrivateKey(keyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, errors.New("JWT private key is not valid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("JWT private key is not RSA")
	}
	if rsaKey.N.BitLen() < 2048 {
		return nil, errors.New("JWT RSA private key must be at least 2048 bits")
	}
	if err := rsaKey.Validate(); err != nil {
		return nil, errors.New("JWT RSA private key is invalid")
	}
	return rsaKey, nil
}

// CreateAccessToken signs an RS256 access token for an authenticated identity.
//
// email is written to both the subject and email claims, provider is written to
// sso_provider, and now is used as the issued-at time. The token expires after
// ttl and carries JWTKeyID in its kid header. An error is returned if RSA
// signing fails.
func CreateAccessToken(key *rsa.PrivateKey, ttl time.Duration, email, provider string, now time.Time) (string, error) {
	claims := accessClaims{
		Email:       email,
		SSOProvider: provider,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "auth_service",
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = JWTKeyID
	return token.SignedString(key)
}

// GenerateRefreshToken returns a cryptographically random URL-safe bearer token.
func GenerateRefreshToken() (string, error) {
	buffer := make([]byte, 48)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

// refreshTokenKey derives the non-reversible Redis key used to store a refresh session.
func refreshTokenKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "refresh_token:" + base64.RawURLEncoding.EncodeToString(digest[:])
}

// PublicJWKS converts key into the single-key JWKS published by the auth service.
//
// The result declares RS256 signing use and uses JWTKeyID, matching the kid
// header produced by CreateAccessToken. The modulus and exponent are encoded
// with unpadded base64url as required by the JWK representation.
func PublicJWKS(key *rsa.PublicKey) map[string]any {
	exponent := key.E
	exponentBytes := make([]byte, 0, 4)
	for exponent > 0 {
		exponentBytes = append([]byte{byte(exponent)}, exponentBytes...)
		exponent >>= 8
	}
	return map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": JWTKeyID,
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(exponentBytes),
	}}}
}
