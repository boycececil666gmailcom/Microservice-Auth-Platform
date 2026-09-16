//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestPasswordAuthAndURLShortenerJourney(t *testing.T) {
	client := newClient(t)
	baseURL := gatewayURL()

	health := request(t, client, http.MethodGet, baseURL+"/health", nil, "")
	assertStatus(t, health, http.StatusOK)
	closeBody(health)

	unauthorized := request(t, client, http.MethodPost, baseURL+"/api/v1/shorten", map[string]string{"long_url": "https://example.com/unauthorized"}, "")
	assertStatus(t, unauthorized, http.StatusUnauthorized)
	closeBody(unauthorized)

	unique := time.Now().UnixNano()
	login := request(t, client, http.MethodPost, baseURL+"/auth/login", map[string]string{
		"email": fmt.Sprintf("testuser_%d@example.com", unique), "password": "testpassword123",
	}, "")
	assertStatus(t, login, http.StatusOK)
	accessToken := decodeToken(t, login)

	longURL := fmt.Sprintf("https://example.com/e2e-%d", unique)
	create := request(t, client, http.MethodPost, baseURL+"/api/v1/shorten", map[string]string{"long_url": longURL}, accessToken)
	assertStatus(t, create, http.StatusCreated)
	var created struct {
		ShortURL int64  `json:"short_url"`
		LongURL  string `json:"long_url"`
	}
	decode(t, create, &created)
	if created.LongURL != longURL || created.ShortURL == 0 {
		t.Fatalf("unexpected short URL response: %#v", created)
	}

	lookup := request(t, client, http.MethodGet, fmt.Sprintf("%s/api/v1/urls/%d", baseURL, created.ShortURL), nil, accessToken)
	assertStatus(t, lookup, http.StatusOK)
	var lookedUp struct {
		LongURL string `json:"long_url"`
	}
	decode(t, lookup, &lookedUp)
	if lookedUp.LongURL != longURL {
		t.Fatalf("looked up %q, want %q", lookedUp.LongURL, longURL)
	}

	initialStats := getStats(t, client, baseURL, accessToken)

	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	redirect := request(t, client, http.MethodGet, fmt.Sprintf("%s/r/%d", baseURL, created.ShortURL), nil, "")
	assertStatus(t, redirect, http.StatusFound)
	if location := redirect.Header.Get("Location"); location != longURL {
		t.Fatalf("redirect location = %q, want %q", location, longURL)
	}
	closeBody(redirect)

	deadline := time.Now().Add(10 * time.Second)
	for {
		updated := getStats(t, client, baseURL, accessToken)
		if updated.TotalRedirects >= initialStats.TotalRedirects+1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("analytics count did not increase: before=%d after=%d", initialStats.TotalRedirects, updated.TotalRedirects)
		}
		time.Sleep(250 * time.Millisecond)
	}

	time.Sleep(time.Second)
	refreshURI, err := url.Parse(baseURL + "/auth/refresh")
	if err != nil {
		t.Fatal(err)
	}
	oldRefreshToken := cookieValue(t, client, refreshURI, "refresh_token")
	refresh := request(t, client, http.MethodPost, baseURL+"/auth/refresh", nil, "")
	assertStatus(t, refresh, http.StatusOK)
	newToken := decodeToken(t, refresh)
	if newToken == accessToken {
		t.Fatal("refresh returned the same access token")
	}
	if newRefreshToken := cookieValue(t, client, refreshURI, "refresh_token"); newRefreshToken == oldRefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	staleRequest, err := http.NewRequest(http.MethodPost, baseURL+"/auth/refresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	staleRequest.AddCookie(&http.Cookie{Name: "refresh_token", Value: oldRefreshToken})
	staleResponse, err := (&http.Client{Timeout: 10 * time.Second}).Do(staleRequest)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, staleResponse, http.StatusUnauthorized)
	closeBody(staleResponse)

	logout := request(t, client, http.MethodPost, baseURL+"/auth/logout", nil, "")
	assertStatus(t, logout, http.StatusOK)
	closeBody(logout)
}

type statsResponse struct {
	TotalRedirects int64 `json:"total_redirects"`
}

func getStats(t *testing.T, client *http.Client, baseURL, token string) statsResponse {
	t.Helper()
	response := request(t, client, http.MethodGet, baseURL+"/api/v1/analytics/stats", nil, token)
	assertStatus(t, response, http.StatusOK)
	var stats statsResponse
	decode(t, response, &stats)
	return stats
}

func TestGoogleOIDCCallbackAndProtectedAccess(t *testing.T) {
	if os.Getenv("RUN_MOCK_OIDC_E2E") != "true" {
		t.Skip("set RUN_MOCK_OIDC_E2E=true against a deployment with ALLOW_MOCK_OIDC=true")
	}
	client := newClient(t)
	baseURL := gatewayURL()
	code := fmt.Sprintf("mock_code_%d", time.Now().UnixNano())
	callback := request(t, client, http.MethodPost, baseURL+"/auth/google/callback", map[string]string{
		"code": code, "state": "test_state_123",
	}, "")
	assertStatus(t, callback, http.StatusOK)
	accessToken := decodeToken(t, callback)
	shorten := request(t, client, http.MethodPost, baseURL+"/api/v1/shorten", map[string]string{
		"long_url": fmt.Sprintf("https://google.com/search?q=%d", time.Now().UnixNano()),
	}, accessToken)
	assertStatus(t, shorten, http.StatusCreated)
	closeBody(shorten)
}

func gatewayURL() string {
	if value := os.Getenv("GATEWAY_URL"); value != "" {
		return value
	}
	return "http://localhost:8000"
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, Timeout: 10 * time.Second}
}

func cookieValue(t *testing.T, client *http.Client, target *url.URL, name string) string {
	t.Helper()
	for _, cookie := range client.Jar.Cookies(target) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	t.Fatalf("cookie %q was not found", name)
	return ""
}

func request(t *testing.T, client *http.Client, method, target string, body any, token string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeToken(t *testing.T, response *http.Response) string {
	t.Helper()
	var body struct {
		AccessToken string `json:"access_token"`
	}
	decode(t, response, &body)
	if body.AccessToken == "" {
		t.Fatal("response is missing access_token")
	}
	return body.AccessToken
}

func decode(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode == want {
		return
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	t.Fatalf("status = %d, want %d: %s", response.StatusCode, want, body)
}

func closeBody(response *http.Response) {
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}
