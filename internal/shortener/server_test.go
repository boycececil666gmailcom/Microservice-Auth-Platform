package shortener

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidHTTPURL(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"https://example.com/path?q=1", true},
		{"http://localhost:8080/path", true},
		{"ftp://example.com/file", false},
		{"https://user:password@example.com/secret", false},
		{"not a URL", false},
		{"/relative", false},
		{"https://example.com/" + string(make([]byte, maxURLLength)), false},
	}
	for _, test := range tests {
		if got := validHTTPURL(test.value); got != test.valid {
			t.Errorf("validHTTPURL(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestShortCodeIsStableAndURLSafe(t *testing.T) {
	first := shortCode("https://example.com/a", 16)
	second := shortCode("https://example.com/a", 16)
	if first != second || !validShortURL(first) {
		t.Fatalf("shortCode returned %q and %q", first, second)
	}
	if first == shortCode("https://example.com/b", 16) {
		t.Fatal("different URLs produced the same test code")
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("URLS_TABLE", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("missing table configuration was accepted")
	}
	t.Setenv("URLS_TABLE", "urls-dev")
	t.Setenv("SQS_QUEUE_URL", "https://sqs.example/queue")
	config, err := ConfigFromEnv()
	if err != nil || config.URLsTable != "urls-dev" || config.SQSQueueURL == "" {
		t.Fatalf("unexpected config: %#v, %v", config, err)
	}
}

func TestShortenLookupAndRedirect(t *testing.T) {
	store := newFakeURLStore()
	publisher := &fakePublisher{}
	handler := newServer(store, publisher).Handler()

	create := serve(handler, http.MethodPost, "/api/v1/shorten", `{"long_url":"https://example.com/article"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", create.Code, create.Body.String())
	}
	var record URLRecord
	if err := json.NewDecoder(create.Body).Decode(&record); err != nil {
		t.Fatal(err)
	}
	if !validShortURL(record.ShortURL) || record.LongURL != "https://example.com/article" {
		t.Fatalf("unexpected record: %#v", record)
	}

	lookup := serve(handler, http.MethodGet, "/api/v1/urls/"+record.ShortURL, "")
	if lookup.Code != http.StatusOK || lookup.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("lookup status = %d", lookup.Code)
	}

	redirect := serve(handler, http.MethodGet, "/r/"+record.ShortURL, "")
	if redirect.Code != http.StatusFound || redirect.Header().Get("Location") != record.LongURL {
		t.Fatalf("unexpected redirect: status=%d location=%q", redirect.Code, redirect.Header().Get("Location"))
	}
	if publisher.lastID != record.ShortURL {
		t.Fatalf("published ID = %q", publisher.lastID)
	}
}

func TestHandlersRejectBadRequestsAndDependencyFailures(t *testing.T) {
	store := newFakeURLStore()
	handler := newServer(store, &fakePublisher{}).Handler()
	tests := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodPost, "/api/v1/shorten", `{"long_url":"javascript:alert(1)"}`, http.StatusUnprocessableEntity},
		{http.MethodPost, "/api/v1/shorten", `{"long_url":"https://example.com","extra":true}`, http.StatusUnprocessableEntity},
		{http.MethodPost, "/api/v1/shorten", `{"long_url":"https://example.com"}{}`, http.StatusUnprocessableEntity},
		{http.MethodGet, "/api/v1/urls/not-valid", "", http.StatusUnprocessableEntity},
		{http.MethodGet, "/api/v1/urls/abcdefghijklmnop", "", http.StatusNotFound},
	}
	for _, test := range tests {
		response := serve(handler, test.method, test.path, test.body)
		if response.Code != test.status {
			t.Errorf("%s %s status = %d, want %d", test.method, test.path, response.Code, test.status)
		}
	}

	store.readyErr = errors.New("unavailable")
	if response := serve(handler, http.MethodGet, "/ready", ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d", response.Code)
	}
	store.readyErr = nil
	store.createErr = errors.New("write failed")
	if response := serve(handler, http.MethodPost, "/api/v1/shorten", `{"long_url":"https://example.com"}`); response.Code != http.StatusInternalServerError {
		t.Fatalf("create failure status = %d", response.Code)
	}
}

func serve(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type fakeURLStore struct {
	records   map[string]URLRecord
	readyErr  error
	createErr error
}

func newFakeURLStore() *fakeURLStore { return &fakeURLStore{records: make(map[string]URLRecord)} }

func (s *fakeURLStore) Create(_ context.Context, longURL string) (URLRecord, error) {
	if s.createErr != nil {
		return URLRecord{}, s.createErr
	}
	code := shortCode(longURL, 16)
	record := URLRecord{ShortURL: code, LongURL: longURL, CreatedAt: time.Unix(1, 0).UTC()}
	s.records[code] = record
	return record, nil
}

func (s *fakeURLStore) Get(_ context.Context, shortURL string) (URLRecord, error) {
	record, ok := s.records[shortURL]
	if !ok {
		return URLRecord{}, ErrNotFound
	}
	return record, nil
}

func (s *fakeURLStore) Ready(context.Context) error { return s.readyErr }

type fakePublisher struct {
	lastID string
	err    error
}

func (p *fakePublisher) PublishRedirect(_ context.Context, shortURL string) error {
	p.lastID = shortURL
	return p.err
}

func TestRequestBodyLimit(t *testing.T) {
	server := newServer(newFakeURLStore(), nil)
	body := `{"long_url":"https://example.com/` + strings.Repeat("a", maxURLLength+300) + `"}`
	response := serve(server.Handler(), http.MethodPost, "/api/v1/shorten", body)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", response.Code)
	}
}
