package shortener

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/httpjson"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

const kafkaTopic = "url-redirects"
const maxURLLength = 2083

type Config struct {
	DatabaseURL string
	RedisURL    string
	KafkaBroker string
	CacheTTL    time.Duration
}

type URLRecord struct {
	ShortURL  int64     `json:"short_url"`
	LongURL   string    `json:"long_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Server struct {
	db             *pgxpool.Pool
	redis          *redis.Client
	kafka          *kafka.Writer
	config         Config
	events         chan int64
	workers        sync.WaitGroup
	producerCtx    context.Context
	cancelProducer context.CancelFunc
}

// ConfigFromEnv loads shortener service configuration from environment variables.
//
// DATABASE_URL and REDIS_URL are required. KAFKA_BROKER_URL defaults to the
// in-cluster Kafka address, while an absent, non-numeric, or non-positive
// CACHE_TTL_SECONDS value defaults to 24 hours.
func ConfigFromEnv() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return Config{}, errors.New("REDIS_URL is required")
	}
	cacheSeconds, err := strconv.Atoi(os.Getenv("CACHE_TTL_SECONDS"))
	if err != nil || cacheSeconds <= 0 {
		cacheSeconds = 24 * 60 * 60
	}
	return Config{
		DatabaseURL: databaseURL,
		RedisURL:    redisURL,
		KafkaBroker: envOr("KAFKA_BROKER_URL", "kafka:9092"),
		CacheTTL:    time.Duration(cacheSeconds) * time.Second,
	}, nil
}

// NewServer initializes the shortener and verifies its external data stores.
//
// It opens and pings PostgreSQL and Redis, creates the urls table when absent,
// configures the Kafka writer, and starts one background analytics publisher.
// If initialization fails before the Server is returned, every resource opened
// by an earlier step is closed.
func NewServer(ctx context.Context, config Config) (*Server, error) {
	db, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	redisOptions, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		db.Close()
		return nil, err
	}
	redisClient := redis.NewClient(redisOptions)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS urls (
			short_url BIGSERIAL PRIMARY KEY,
			long_url TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		db.Close()
		_ = redisClient.Close()
		return nil, err
	}
	writer := &kafka.Writer{
		Addr: kafka.TCP(config.KafkaBroker), Topic: kafkaTopic,
		Balancer: &kafka.LeastBytes{}, RequiredAcks: kafka.RequireOne,
	}
	producerCtx, cancelProducer := context.WithCancel(context.Background())
	server := &Server{
		db: db, redis: redisClient, kafka: writer, config: config,
		events: make(chan int64, 1024), producerCtx: producerCtx, cancelProducer: cancelProducer,
	}
	server.workers.Add(1)
	go server.publishAnalytics()
	return server, nil
}

// Close stops event publishing and releases all external resources.
//
// Canceling producerCtx makes the publisher stop promptly rather than flushing
// every queued event. Close then closes the queue, waits for the worker, and
// closes PostgreSQL, Redis, and Kafka. It must be called only once and only after
// request handlers have stopped enqueueing events.
func (s *Server) Close() {
	s.cancelProducer()
	close(s.events)
	s.workers.Wait()
	s.db.Close()
	_ = s.redis.Close()
	_ = s.kafka.Close()
}

// Handler returns the HTTP handler containing all shortener service routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("POST /shorten", s.shorten)
	mux.HandleFunc("GET /urls/{shortURL}", s.lookup)
	mux.HandleFunc("GET /r/{shortURL}", s.redirect)
	return mux
}

// health reports that the shortener process is running.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready checks PostgreSQL and Redis before reporting readiness.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		httpjson.Error(w, http.StatusServiceUnavailable, "Database unavailable")
		return
	}
	if err := s.redis.Ping(ctx).Err(); err != nil {
		httpjson.Error(w, http.StatusServiceUnavailable, "Cache unavailable")
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ready"})
}

// shorten creates or retrieves the stable short record for a destination URL.
//
// The JSON body must contain an absolute, credential-free HTTP(S) URL within the
// configured size limit. PostgreSQL's unique constraint makes repeated requests
// for the same destination reuse its identifier. After reading the stored row,
// the handler attempts to pre-warm Redis; a cache failure is logged but does not
// fail the successful database operation or response.
func (s *Server) shorten(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LongURL string `json:"long_url"`
	}
	if err := httpjson.Decode(w, r, &body); err != nil || !validHTTPURL(body.LongURL) {
		httpjson.Error(w, http.StatusUnprocessableEntity, "Invalid long_url")
		return
	}
	if _, err := s.db.Exec(r.Context(),
		"INSERT INTO urls (long_url) VALUES ($1) ON CONFLICT (long_url) DO NOTHING", body.LongURL,
	); err != nil {
		slog.Error("URL insert failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	record, err := s.getByLongURL(r.Context(), body.LongURL)
	if err != nil {
		slog.Error("URL lookup after insert failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := s.setCached(r.Context(), record); err != nil {
		slog.Warn("URL cache pre-warm failed", "error", err)
	}
	httpjson.Write(w, http.StatusCreated, record)
}

// lookup resolves a short URL identifier and returns its stored URL record.
func (s *Server) lookup(w http.ResponseWriter, r *http.Request) {
	record, status, err := s.resolve(r.Context(), r.PathValue("shortURL"))
	if err != nil {
		httpjson.Error(w, status, errorDetail(status))
		return
	}
	httpjson.Write(w, http.StatusOK, record)
}

// redirect resolves a short URL, queues an analytics event, and redirects the client.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	record, status, err := s.resolve(r.Context(), r.PathValue("shortURL"))
	if err != nil {
		httpjson.Error(w, status, errorDetail(status))
		return
	}
	s.enqueueAnalytics(record.ShortURL)
	http.Redirect(w, r, record.LongURL, http.StatusFound)
}

// resolve validates a short identifier and loads its URL record.
//
// A positive base-10 integer is required. Redis is checked first; cache misses,
// malformed entries, and Redis failures fall back to PostgreSQL. A successful
// database lookup is written back to the cache on a best-effort basis.
//
// On success, status is zero. On failure, status is the HTTP status the caller
// should send: 422 for an invalid identifier, 404 for a missing row, or 500 for
// another database failure. The accompanying error is intended for control flow
// and logging, not direct disclosure to clients.
func (s *Server) resolve(ctx context.Context, value string) (URLRecord, int, error) {
	shortURL, err := strconv.ParseInt(value, 10, 64)
	if err != nil || shortURL <= 0 {
		return URLRecord{}, http.StatusUnprocessableEntity, errors.New("invalid short URL")
	}
	record, err := s.getCached(ctx, shortURL)
	if err == nil {
		return record, 0, nil
	}
	if !errors.Is(err, redis.Nil) {
		slog.Warn("URL cache lookup failed; using PostgreSQL", "error", err)
	}
	record, err = s.getByID(ctx, shortURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return URLRecord{}, http.StatusNotFound, err
	}
	if err != nil {
		return URLRecord{}, http.StatusInternalServerError, err
	}
	if err := s.setCached(ctx, record); err != nil {
		slog.Warn("URL cache warm failed", "error", err)
	}
	return record, 0, nil
}

// getByLongURL loads a URL record from PostgreSQL by its destination URL.
func (s *Server) getByLongURL(ctx context.Context, longURL string) (URLRecord, error) {
	var record URLRecord
	err := s.db.QueryRow(ctx,
		"SELECT short_url, long_url, created_at FROM urls WHERE long_url = $1", longURL,
	).Scan(&record.ShortURL, &record.LongURL, &record.CreatedAt)
	return record, err
}

// getByID loads a URL record from PostgreSQL by its numeric short URL.
func (s *Server) getByID(ctx context.Context, shortURL int64) (URLRecord, error) {
	var record URLRecord
	err := s.db.QueryRow(ctx,
		"SELECT short_url, long_url, created_at FROM urls WHERE short_url = $1", shortURL,
	).Scan(&record.ShortURL, &record.LongURL, &record.CreatedAt)
	return record, err
}

// getCached loads and decodes a URL record from Redis.
//
// redis.Nil is returned unchanged so callers can distinguish a cache miss. If a
// cached value is not valid URLRecord JSON, the corrupt entry is deleted on a
// best-effort basis and the decoding error is returned.
func (s *Server) getCached(ctx context.Context, shortURL int64) (URLRecord, error) {
	raw, err := s.redis.Get(ctx, cacheKey(shortURL)).Bytes()
	if err != nil {
		return URLRecord{}, err
	}
	var record URLRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		_ = s.redis.Del(ctx, cacheKey(shortURL)).Err()
		return URLRecord{}, err
	}
	return record, nil
}

// setCached encodes a URL record and stores it in Redis for the configured TTL.
func (s *Server) setCached(ctx context.Context, record URLRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, cacheKey(record.ShortURL), payload, s.config.CacheTTL).Err()
}

// enqueueAnalytics attempts to queue a redirect event without blocking the request.
//
// If the bounded channel is full, the event is deliberately dropped and a
// warning is logged. Redirect availability therefore does not depend on Kafka
// throughput, at the cost of potentially incomplete analytics during overload.
func (s *Server) enqueueAnalytics(shortURL int64) {
	select {
	case s.events <- shortURL:
	default:
		slog.Warn("analytics event queue full; dropping event", "short_url", shortURL)
	}
}

// publishAnalytics drains redirect events and publishes them to Kafka.
//
// Each event is encoded as a redirect message and attempted at most three times.
// Every write has a five-second timeout, with exponential backoff beginning at
// 250 milliseconds between failures. Cancellation stops an active retry loop and
// exits without draining the remaining queue. The worker records completion in
// s.workers so Close can wait for it.
func (s *Server) publishAnalytics() {
	defer s.workers.Done()
	for {
		var shortURL int64
		select {
		case <-s.producerCtx.Done():
			return
		case value, ok := <-s.events:
			if !ok {
				return
			}
			shortURL = value
		}
		payload, err := json.Marshal(map[string]any{"short_url": shortURL, "event": "redirect"})
		if err != nil {
			slog.Error("analytics event serialization failed", "error", err)
			continue
		}
		for attempt := 1; attempt <= 3; attempt++ {
			ctx, cancel := context.WithTimeout(s.producerCtx, 5*time.Second)
			err = s.kafka.WriteMessages(ctx, kafka.Message{Value: payload})
			cancel()
			if err == nil || s.producerCtx.Err() != nil {
				break
			}
			backoff := time.Duration(1<<(attempt-1)) * 250 * time.Millisecond
			select {
			case <-s.producerCtx.Done():
				return
			case <-time.After(backoff):
			}
		}
		if err != nil && s.producerCtx.Err() == nil {
			slog.Warn("Kafka analytics event failed after retries", "error", err)
		}
	}
}

// cacheKey returns the Redis key for a numeric short URL.
func cacheKey(shortURL int64) string { return "url:" + strconv.FormatInt(shortURL, 10) }

// validHTTPURL reports whether raw is an absolute, credential-free HTTP(S) URL within the size limit.
func validHTTPURL(raw string) bool {
	if len(raw) == 0 || len(raw) > maxURLLength {
		return false
	}
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https")
}

// errorDetail maps a resolution status to its public error message.
func errorDetail(status int) string {
	if status == http.StatusNotFound {
		return "Short URL not found"
	}
	if status == http.StatusUnprocessableEntity {
		return "Invalid short URL"
	}
	return "Internal server error"
}

// envOr returns the named environment variable or fallback when it is unset.
func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
