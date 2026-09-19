package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const googleAuthEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"

type GoogleUser struct {
	Email     string
	GoogleSub string
	Name      string
	Picture   string
}

// GenerateStateToken returns a cryptographically random URL-safe OAuth state value.
func GenerateStateToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// BuildGoogleAuthURL constructs a Google OAuth authorization URL.
//
// clientID and redirectURI are copied into the query without validation. When
// state is empty, the function generates a cryptographically random state value;
// callers must retain that value from the returned URL and verify it during the
// callback. An error is returned only when state generation fails.
func BuildGoogleAuthURL(clientID, redirectURI, state string) (string, error) {
	if state == "" {
		var err error
		state, err = GenerateStateToken()
		if err != nil {
			return "", err
		}
	}
	query := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"access_type":   {"offline"},
		"state":         {state},
		"prompt":        {"consent"},
	}
	return googleAuthEndpoint + "?" + query.Encode(), nil
}

// ExchangeGoogleCode exchanges a one-time Google authorization code for an ID token.
//
// code must be non-empty. redirectURI must match the URI used for authorization;
// when it is empty, cfg.GoogleCallbackURL is used. The function submits the code
// and client credentials to cfg.GoogleTokenURL with client, limits the response
// body to 1 MiB, and returns the id_token field from a successful response.
// Transport failures, non-200 responses, malformed JSON, and a missing ID token
// are returned as errors.
//
// When mock OIDC is explicitly enabled, the function performs no network request
// and returns a locally generated test token derived from code.
func ExchangeGoogleCode(ctx context.Context, client *http.Client, cfg Config, code, redirectURI string) (string, error) {
	if code == "" {
		return "", errors.New("authorization code must not be empty")
	}
	if isMockGoogleConfig(cfg) {
		email := "mock_google@gmail.com"
		if len(code) >= 6 {
			email = "user_" + code[len(code)-6:] + "@gmail.com"
		}
		return BuildMockGoogleIDToken(cfg.GoogleClientID, email, "google_sub_"+code, "Mock Google User")
	}
	if redirectURI == "" {
		redirectURI = cfg.GoogleCallbackURL
	}
	form := url.Values{
		"code":          {code},
		"client_id":     {cfg.GoogleClientID},
		"client_secret": {cfg.GoogleClientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.GoogleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to exchange code with Google Token Endpoint: %s", strings.TrimSpace(string(body)))
	}
	var payload struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.IDToken == "" {
		return "", errors.New("Google token endpoint response did not include 'id_token'")
	}
	return payload.IDToken, nil
}

// ParseMockGoogleIDToken extracts a GoogleUser from a locally generated mock token.
//
// It requires a three-part JWT-shaped value, decodes the payload, and validates
// the email, subject, and email_verified claims. Missing names are derived from
// the email address. The placeholder signature, issuer, audience, and expiry are
// intentionally not verified, so this function must only be used behind the
// explicit mock-OIDC configuration guard.
func ParseMockGoogleIDToken(idToken string) (GoogleUser, error) {
	if strings.TrimSpace(idToken) == "" {
		return GoogleUser{}, errors.New("invalid Google ID Token format")
	}
	parts := strings.Split(strings.TrimSpace(idToken), ".")
	if len(parts) != 3 {
		return GoogleUser{}, errors.New("malformed Google ID Token format (expected JWT header.payload.signature)")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return GoogleUser{}, fmt.Errorf("failed to decode Google ID Token payload: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return GoogleUser{}, fmt.Errorf("invalid JSON in ID Token: %w", err)
	}
	email, emailOK := payload["email"].(string)
	subValue, subOK := payload["sub"]
	sub := ""
	if subOK {
		sub = fmt.Sprint(subValue)
	}
	if !emailOK || email == "" || !subOK || sub == "" {
		return GoogleUser{}, errors.New("Google ID Token payload missing required 'email' or 'sub' claims")
	}
	name, _ := payload["name"].(string)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	picture, _ := payload["picture"].(string)
	verified, _ := payload["email_verified"].(bool)
	if !verified {
		return GoogleUser{}, errors.New("Google ID Token email is not verified")
	}
	return GoogleUser{Email: email, GoogleSub: sub, Name: name, Picture: picture}, nil
}

// isMockGoogleConfig reports whether the explicit mock OIDC safeguards are both enabled.
func isMockGoogleConfig(cfg Config) bool {
	return cfg.AllowMockOIDC && strings.HasPrefix(cfg.GoogleClientID, "mock-")
}

// BuildMockGoogleIDToken creates a test-only Google-shaped ID token.
//
// The token contains the supplied audience and identity claims, a one-hour
// lifetime, and a placeholder signature that is not cryptographically valid.
// It must never be accepted by the production GoogleVerifier.
func BuildMockGoogleIDToken(clientID, email, sub, name string) (string, error) {
	header, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": "mock_google_key"})
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(map[string]any{
		"iss": "https://accounts.google.com", "azp": clientID, "aud": clientID,
		"sub": sub, "email": email, "email_verified": true, "name": name,
		"picture": "https://lh3.googleusercontent.com/a/mock_photo", "iat": now, "exp": now + 3600,
	})
	if err != nil {
		return "", err
	}
	signature := base64.RawURLEncoding.EncodeToString([]byte("mock_google_rsa_signature"))
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + signature, nil
}
