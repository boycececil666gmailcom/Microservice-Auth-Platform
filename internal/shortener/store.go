package shortener

// #region Store Operations

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

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
		slog.Warn("[Shortener-resolve] URL cache lookup failed; using PostgreSQL", "error", err)
	}
	record, err = s.getByID(ctx, shortURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return URLRecord{}, http.StatusNotFound, err
	}
	if err != nil {
		return URLRecord{}, http.StatusInternalServerError, err
	}
	if err := s.setCached(ctx, record); err != nil {
		slog.Warn("[Shortener-resolve] URL cache warm failed", "error", err)
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

// cacheKey returns the Redis key for a numeric short URL.
func cacheKey(shortURL int64) string { return "url:" + strconv.FormatInt(shortURL, 10) }

// #endregion
