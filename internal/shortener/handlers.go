package shortener

// #region HTTP Handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/boycececil666gmailcom/Microservice-Auth-Platform/internal/httpjson"
)

const maxURLLength = 2083

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
		slog.Error("[Shortener-shorten] URL insert failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	record, err := s.getByLongURL(r.Context(), body.LongURL)
	if err != nil {
		slog.Error("[Shortener-shorten] URL lookup after insert failed", "error", err)
		httpjson.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := s.setCached(r.Context(), record); err != nil {
		slog.Warn("[Shortener-shorten] URL cache pre-warm failed", "error", err)
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

// #endregion
