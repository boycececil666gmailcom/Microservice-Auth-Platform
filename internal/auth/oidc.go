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

func GenerateStateToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

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

func isMockGoogleConfig(cfg Config) bool {
	return cfg.AllowMockOIDC && strings.HasPrefix(cfg.GoogleClientID, "mock-")
}

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
