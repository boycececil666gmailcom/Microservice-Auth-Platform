package auth

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestConfigFromEnvRequiresPrivateKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/auth")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_PRIVATE_KEY", "")
	t.Setenv("RSA_PRIVATE_KEY_PEM", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("missing private key was accepted")
	}
}

func TestConfigFromEnvReadsSecuritySettings(t *testing.T) {
	key := testPrivateKey(t)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_PRIVATE_KEY", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})))
	t.Setenv("DATABASE_URL", "postgresql://localhost/auth")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("JWT_EXPIRATION_MINUTES", "20")
	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !config.CookieSecure || config.AccessTokenTTL.Minutes() != 20 {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestNormalizeEmailAndPasswordLimits(t *testing.T) {
	if email, ok := normalizeEmail(" Alice@Example.COM "); !ok || email != "alice@example.com" {
		t.Fatalf("normalizeEmail returned %q, %v", email, ok)
	}
	for _, invalid := range []string{"", "not-an-email", "Name <alice@example.com>"} {
		if _, ok := normalizeEmail(invalid); ok {
			t.Fatalf("invalid email %q was accepted", invalid)
		}
	}
}
