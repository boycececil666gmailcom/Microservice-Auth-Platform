package shortener

// #region Server Lifecycle

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	SQSQueueURL string
	CacheTTL    time.Duration
}

type URLRecord struct {
	ShortURL  int64     `json:"short_url"`
	LongURL   string    `json:"long_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Server struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	sqs    *sqs.Client
	config Config
}

// ConfigFromEnv loads shortener service configuration from environment variables.
//
// DATABASE_URL and REDIS_URL are required. SQS_QUEUE_URL is read for asynchronous
// redirect analytics event publishing. An absent, non-numeric, or non-positive
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
		SQSQueueURL: os.Getenv("SQS_QUEUE_URL"),
		CacheTTL:    time.Duration(cacheSeconds) * time.Second,
	}, nil
}

// NewServer initializes the shortener and verifies its external data stores.
//
// It opens and pings PostgreSQL and Redis, creates the urls table when absent,
// and initializes the AWS SQS client.
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

	var sqsClient *sqs.Client
	if config.SQSQueueURL != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err == nil {
			sqsClient = sqs.NewFromConfig(awsCfg)
		}
	}

	return &Server{
		db: db, redis: redisClient, sqs: sqsClient, config: config,
	}, nil
}

// Close releases all external resources.
func (s *Server) Close() {
	s.db.Close()
	_ = s.redis.Close()
}

// Handler returns the HTTP handler containing all shortener service routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// Core routes
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.HandleFunc("POST /shorten", s.shorten)
	mux.HandleFunc("GET /urls/{shortURL}", s.lookup)
	mux.HandleFunc("GET /r/{shortURL}", s.redirect)

	// API v1 prefixed routes for AWS API Gateway integration
	mux.HandleFunc("POST /api/v1/shorten", s.shorten)
	mux.HandleFunc("GET /api/v1/urls/{shortURL}", s.lookup)
	return mux
}

// #endregion
