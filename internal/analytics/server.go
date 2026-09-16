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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /stats", s.getStats)
	return mux
}

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

func (s *Server) Close() error { return s.reader.Close() }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) recordRedirect(shortURL int64) {
	s.mu.Lock()
	s.stats.TotalRedirects++
	s.stats.RedirectsByShortURL[shortURL]++
	s.mu.Unlock()
}

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
