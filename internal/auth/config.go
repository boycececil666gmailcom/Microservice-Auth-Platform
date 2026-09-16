package auth

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL        string
	RedisURL           string
	PrivateKeyPEM      string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	CookieSecure       bool
	AllowMockOIDC      bool
	GoogleClientID     string
	GoogleClientSecret string
	GoogleCallbackURL  string
	GoogleTokenURL     string
	GoogleJWKSURL      string
}

func ConfigFromEnv() (Config, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return Config{}, errors.New("REDIS_URL is required")
	}
	privateKey := firstNonEmpty(os.Getenv("JWT_PRIVATE_KEY"), os.Getenv("RSA_PRIVATE_KEY_PEM"))
	if strings.TrimSpace(privateKey) == "" {
		return Config{}, errors.New("JWT_PRIVATE_KEY or RSA_PRIVATE_KEY_PEM is required")
	}
	return Config{
		DatabaseURL:        databaseURL,
		RedisURL:           redisURL,
		PrivateKeyPEM:      privateKey,
		AccessTokenTTL:     time.Duration(envInt("JWT_EXPIRATION_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL:    time.Duration(envInt("REFRESH_TOKEN_TTL_SECONDS", 30*24*60*60)) * time.Second,
		CookieSecure:       envBool("COOKIE_SECURE", false),
		AllowMockOIDC:      envBool("ALLOW_MOCK_OIDC", false),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleCallbackURL:  envOr("GOOGLE_OIDC_CALLBACK_TO_BACKEND_URL", "http://localhost/auth/google/callback"),
		GoogleTokenURL:     envOr("GOOGLE_TOKEN_ENDPOINT", "https://oauth2.googleapis.com/token"),
		GoogleJWKSURL:      envOr("GOOGLE_JWKS_ENDPOINT", "https://www.googleapis.com/oauth2/v3/certs"),
	}, nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
