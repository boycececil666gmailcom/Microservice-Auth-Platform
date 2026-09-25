package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("ANALYTICS_TABLE", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("missing table was accepted")
	}
	t.Setenv("ANALYTICS_TABLE", "analytics-dev")
	t.Setenv("DEDUPE_TTL_SECONDS", "1296000")
	config, err := ConfigFromEnv()
	if err != nil || config.DedupeTTL != 15*24*time.Hour {
		t.Fatalf("unexpected config: %#v, %v", config, err)
	}
	t.Setenv("DEDUPE_TTL_SECONDS", "10")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("unsafe dedupe TTL was accepted")
	}
}

func TestStatsHandlerReturnsSnapshot(t *testing.T) {
	store := newFakeCounterStore()
	store.stats = Stats{TotalRedirects: 2, RedirectsByShortURL: map[string]int64{"abcdefghijklmnop": 2}}
	server := newServer(store, 7*24*time.Hour)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/analytics/stats", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var stats Stats
	if err := json.NewDecoder(response.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.TotalRedirects != 2 || stats.RedirectsByShortURL["abcdefghijklmnop"] != 2 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestHandlersExposeDependencyFailures(t *testing.T) {
	store := newFakeCounterStore()
	server := newServer(store, 24*time.Hour)
	store.err = errors.New("unavailable")
	for _, path := range []string{"/ready", "/api/v1/analytics/stats"} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d", path, response.Code)
		}
	}
}

func TestProcessSQSEventReportsOnlyFailedItems(t *testing.T) {
	store := newFakeCounterStore()
	server := newServer(store, 24*time.Hour)
	validID := "abcdefghijklmnop"
	event := events.SQSEvent{Records: []events.SQSMessage{
		{MessageId: "ok", Body: `{"short_url":"` + validID + `","event":"redirect"}`},
		{MessageId: "bad-json", Body: `{`},
		{MessageId: "bad-event", Body: `{"short_url":"` + validID + `","event":"unknown"}`},
	}}
	response := server.ProcessSQSEvent(context.Background(), event)
	if len(response.BatchItemFailures) != 2 {
		t.Fatalf("failures = %#v", response.BatchItemFailures)
	}
	if store.stats.TotalRedirects != 1 || store.stats.RedirectsByShortURL[validID] != 1 {
		t.Fatalf("unexpected stats: %#v", store.stats)
	}
}

func TestRecordRedirectIsIdempotentAndConcurrencySafe(t *testing.T) {
	store := newFakeCounterStore()
	server := newServer(store, 24*time.Hour)
	const count = 100
	var wait sync.WaitGroup
	for index := range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := server.RecordRedirect(context.Background(), "abcdefghijklmnop", string(rune(index))); err != nil {
				t.Errorf("RecordRedirect: %v", err)
			}
		}()
	}
	wait.Wait()
	if err := server.RecordRedirect(context.Background(), "abcdefghijklmnop", string(rune(1))); err != nil {
		t.Fatal(err)
	}
	if store.stats.TotalRedirects != count {
		t.Fatalf("total = %d", store.stats.TotalRedirects)
	}
}

type fakeCounterStore struct {
	mu     sync.Mutex
	stats  Stats
	events map[string]struct{}
	err    error
}

func newFakeCounterStore() *fakeCounterStore {
	return &fakeCounterStore{
		stats:  Stats{RedirectsByShortURL: make(map[string]int64)},
		events: make(map[string]struct{}),
	}
}

func (s *fakeCounterStore) Increment(_ context.Context, shortURL, eventID string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if _, exists := s.events[eventID]; exists {
		return nil
	}
	s.events[eventID] = struct{}{}
	s.stats.TotalRedirects++
	s.stats.RedirectsByShortURL[shortURL]++
	return nil
}

func (s *fakeCounterStore) Snapshot(context.Context) (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return Stats{}, s.err
	}
	copyStats := Stats{TotalRedirects: s.stats.TotalRedirects, RedirectsByShortURL: make(map[string]int64)}
	for key, value := range s.stats.RedirectsByShortURL {
		copyStats.RedirectsByShortURL[key] = value
	}
	return copyStats, nil
}

func (s *fakeCounterStore) Ready(context.Context) error { return s.err }
