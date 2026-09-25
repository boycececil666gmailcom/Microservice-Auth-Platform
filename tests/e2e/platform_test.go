//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestURLShortenerAndAnalyticsJourney(t *testing.T) {
	client := newClient(t)
	baseURL := gatewayURL()

	health := request(t, client, http.MethodGet, baseURL+"/health", nil, "")
	assertStatus(t, health, http.StatusOK)
	closeBody(health)

	unique := time.Now().UnixNano()
	longURL := fmt.Sprintf("https://example.com/e2e-%d", unique)
	create := request(t, client, http.MethodPost, baseURL+"/api/v1/shorten", map[string]string{"long_url": longURL}, "")
	assertStatus(t, create, http.StatusCreated)
	var created struct {
		ShortURL string `json:"short_url"`
		LongURL  string `json:"long_url"`
	}
	decode(t, create, &created)
	if created.LongURL != longURL || created.ShortURL == "" {
		t.Fatalf("unexpected short URL response: %#v", created)
	}

	lookup := request(t, client, http.MethodGet, fmt.Sprintf("%s/api/v1/urls/%s", baseURL, created.ShortURL), nil, "")
	assertStatus(t, lookup, http.StatusOK)
	var lookedUp struct {
		LongURL string `json:"long_url"`
	}
	decode(t, lookup, &lookedUp)
	if lookedUp.LongURL != longURL {
		t.Fatalf("looked up %q, want %q", lookedUp.LongURL, longURL)
	}

	initialStats := getStats(t, client, baseURL)

	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	redirect := request(t, client, http.MethodGet, fmt.Sprintf("%s/r/%s", baseURL, created.ShortURL), nil, "")
	assertStatus(t, redirect, http.StatusFound)
	if location := redirect.Header.Get("Location"); location != longURL {
		t.Fatalf("redirect location = %q, want %q", location, longURL)
	}
	closeBody(redirect)

	deadline := time.Now().Add(10 * time.Second)
	for {
		updated := getStats(t, client, baseURL)
		if updated.TotalRedirects >= initialStats.TotalRedirects+1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("analytics count did not increase: before=%d after=%d", initialStats.TotalRedirects, updated.TotalRedirects)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

type statsResponse struct {
	TotalRedirects int64 `json:"total_redirects"`
}

func getStats(t *testing.T, client *http.Client, baseURL string) statsResponse {
	t.Helper()
	response := request(t, client, http.MethodGet, baseURL+"/api/v1/analytics/stats", nil, "")
	assertStatus(t, response, http.StatusOK)
	var stats statsResponse
	decode(t, response, &stats)
	return stats
}

func gatewayURL() string {
	if value := os.Getenv("GATEWAY_URL"); value != "" {
		return value
	}
	return "http://localhost:8000"
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{Timeout: 10 * time.Second}
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
