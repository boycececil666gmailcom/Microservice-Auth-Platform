package analytics

// #region Analytics Server

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/httpjson"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	RedisURL string
}

type Stats struct {
	TotalRedirects      int64           `json:"total_redirects"`
	RedirectsByShortURL map[int64]int64 `json:"redirects_by_short_url"`
}

type Server struct {
	redis  *redis.Client
	config Config
	mu     sync.RWMutex
	stats  Stats
}

// ConfigFromEnv loads analytics configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		RedisURL: os.Getenv("REDIS_URL"),
	}
}

// NewServer creates an analytics service using environment configuration.
func NewServer() *Server {
	return NewServerWithConfig(context.Background(), ConfigFromEnv())
}

// NewServerWithConfig initializes analytics with the provided configuration.
func NewServerWithConfig(ctx context.Context, config Config) *Server {
	var redisClient *redis.Client
	if config.RedisURL != "" {
		if opts, err := redis.ParseURL(config.RedisURL); err == nil {
			redisClient = redis.NewClient(opts)
		}
	}

	return &Server{
		redis:  redisClient,
		config: config,
		stats:  Stats{RedirectsByShortURL: make(map[int64]int64)},
	}
}

// Handler returns the HTTP handler containing all analytics service routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("GET /stats", s.getStats)
	mux.HandleFunc("GET /api/v1/analytics/stats", s.getStats)
	return mux
}

// Close releases external connections used by the analytics service.
func (s *Server) Close() error {
	if s.redis != nil {
		return s.redis.Close()
	}
	return nil
}

// health reports that the analytics process is running.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready reports that the analytics service is ready.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.redis != nil {
		if err := s.redis.Ping(r.Context()).Err(); err != nil {
			httpjson.Error(w, http.StatusServiceUnavailable, "Cache unavailable")
			return
		}
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ready"})
}

// RecordRedirect increments redirect counters in Redis and in-memory.
func (s *Server) RecordRedirect(shortURL int64) {
	s.recordRedirect(shortURL)
}

// recordRedirect increments redirect counters in Redis and in-memory.
func (s *Server) recordRedirect(shortURL int64) {
	if s.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.redis.Incr(ctx, "analytics:total_redirects").Err()
		_ = s.redis.HIncrBy(ctx, "analytics:redirects_by_short_url", strconv.FormatInt(shortURL, 10), 1).Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.TotalRedirects++
	if s.stats.RedirectsByShortURL == nil {
		s.stats.RedirectsByShortURL = make(map[int64]int64)
	}
	s.stats.RedirectsByShortURL[shortURL]++
}

// getStats returns current redirect counters from Redis or in-memory fallback.
func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	if s.redis != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		totalStr, err := s.redis.Get(ctx, "analytics:total_redirects").Result()
		var total int64
		if err == nil {
			total, _ = strconv.ParseInt(totalStr, 10, 64)
		}

		rawMap, err := s.redis.HGetAll(ctx, "analytics:redirects_by_short_url").Result()
		byShortURL := make(map[int64]int64, len(rawMap))
		if err == nil {
			for k, v := range rawMap {
				id, err1 := strconv.ParseInt(k, 10, 64)
				count, err2 := strconv.ParseInt(v, 10, 64)
				if err1 == nil && err2 == nil {
					byShortURL[id] = count
				}
			}
		}
		httpjson.Write(w, http.StatusOK, Stats{
			TotalRedirects:      total,
			RedirectsByShortURL: byShortURL,
		})
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[int64]int64, len(s.stats.RedirectsByShortURL))
	for k, v := range s.stats.RedirectsByShortURL {
		copyMap[k] = v
	}
	httpjson.Write(w, http.StatusOK, Stats{
		TotalRedirects:      s.stats.TotalRedirects,
		RedirectsByShortURL: copyMap,
	})
}

// #endregion
