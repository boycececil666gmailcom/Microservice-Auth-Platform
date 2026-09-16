package analytics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsHandlerReturnsSnapshot(t *testing.T) {
	server := &Server{stats: Stats{TotalRedirects: 2, RedirectsByShortURL: map[int64]int64{7: 2}}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/stats", nil)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var stats Stats
	if err := json.NewDecoder(recorder.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.TotalRedirects != 2 || stats.RedirectsByShortURL[7] != 2 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestRecordRedirectIsConcurrencySafe(t *testing.T) {
	server := &Server{stats: Stats{RedirectsByShortURL: make(map[int64]int64)}}
	const count = 100
	done := make(chan struct{}, count)
	for range count {
		go func() {
			server.recordRedirect(7)
			done <- struct{}{}
		}()
	}
	for range count {
		<-done
	}
	if server.stats.TotalRedirects != count || server.stats.RedirectsByShortURL[7] != count {
		t.Fatalf("unexpected stats: %#v", server.stats)
	}
}
