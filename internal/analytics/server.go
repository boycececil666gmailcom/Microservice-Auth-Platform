package analytics

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/httpjson"
	"github.com/segmentio/kafka-go"
)

const kafkaTopic = "url-redirects"

type Stats struct {
	TotalRedirects      int64           `json:"total_redirects"`
	RedirectsByShortURL map[int64]int64 `json:"redirects_by_short_url"`
}

type Server struct {
	reader *kafka.Reader
	mu     sync.RWMutex
	stats  Stats
}

// NewServer creates an analytics service configured to consume redirect events from Kafka.
func NewServer() *Server {
	broker := os.Getenv("KAFKA_BROKER_URL")
	if broker == "" {
		broker = "kafka:9092"
	}
	return &Server{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{broker}, Topic: kafkaTopic, GroupID: "analytics-group",
			StartOffset: kafka.FirstOffset, MinBytes: 1, MaxBytes: 10e6,
		}),
		stats: Stats{RedirectsByShortURL: make(map[int64]int64)},
	}
}

// Handler returns the HTTP handler containing all analytics service routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /stats", s.getStats)
	return mux
}

// Consume reads Kafka redirect events until ctx is canceled.
//
// Read failures are logged and retried after two seconds unless cancellation is
// in progress. Each message must be valid JSON with event set to "redirect" and
// a positive short_url; unsupported or malformed messages are logged and
// skipped. Accepted events update the concurrency-safe in-memory counters.
func (s *Server) Consume(ctx context.Context) {
	for {
		message, err := s.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("Kafka read failed", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		var event struct {
			ShortURL int64  `json:"short_url"`
			Event    string `json:"event"`
		}
		if err := json.Unmarshal(message.Value, &event); err != nil {
			slog.Warn("invalid Kafka analytics event", "error", err)
			continue
		}
		if event.Event != "redirect" || event.ShortURL <= 0 {
			slog.Warn("ignoring unsupported Kafka analytics event", "event", event.Event, "short_url", event.ShortURL)
			continue
		}
		s.recordRedirect(event.ShortURL)
	}
}

// Close releases the Kafka reader used by the service.
func (s *Server) Close() error { return s.reader.Close() }

// health reports that the analytics process is running.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready reports that the analytics HTTP service is ready to receive requests.
func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ready"})
}

// recordRedirect increments the aggregate and per-short-URL redirect counts safely.
func (s *Server) recordRedirect(shortURL int64) {
	s.mu.Lock()
	s.stats.TotalRedirects++
	s.stats.RedirectsByShortURL[shortURL]++
	s.mu.Unlock()
}

// getStats returns a consistent snapshot of the current redirect counters.
//
// The map is copied while holding a read lock so JSON encoding can proceed after
// the lock is released without racing with the Kafka consumer.
func (s *Server) getStats(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	copy := Stats{
		TotalRedirects:      s.stats.TotalRedirects,
		RedirectsByShortURL: make(map[int64]int64, len(s.stats.RedirectsByShortURL)),
	}
	for key, value := range s.stats.RedirectsByShortURL {
		copy.RedirectsByShortURL[key] = value
	}
	s.mu.RUnlock()
	httpjson.Write(w, http.StatusOK, copy)
}
