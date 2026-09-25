package shortener

// #region HTTP Handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const maxURLLength = 2083

// health reports that the shortener process is running.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready checks PostgreSQL and Redis before reporting readiness.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		errorJSON(w, http.StatusServiceUnavailable, "Database unavailable")
		return
	}
	if err := s.redis.Ping(ctx).Err(); err != nil {
		errorJSON(w, http.StatusServiceUnavailable, "Cache unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validHTTPURL(body.LongURL) {
		errorJSON(w, http.StatusUnprocessableEntity, "Invalid long_url")
		return
	}
	if _, err := s.db.Exec(r.Context(),
		"INSERT INTO urls (long_url) VALUES ($1) ON CONFLICT (long_url) DO NOTHING", body.LongURL,
	); err != nil {
		slog.Error("[Shortener-shorten] URL insert failed", "error", err)
		errorJSON(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	record, err := s.getByLongURL(r.Context(), body.LongURL)
	if err != nil {
		slog.Error("[Shortener-shorten] URL lookup after insert failed", "error", err)
		errorJSON(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if err := s.setCached(r.Context(), record); err != nil {
		slog.Warn("[Shortener-shorten] URL cache pre-warm failed", "error", err)
	}
	writeJSON(w, http.StatusCreated, record)
}

// lookup resolves a short URL identifier and returns its stored URL record.
func (s *Server) lookup(w http.ResponseWriter, r *http.Request) {
	record, status, err := s.resolve(r.Context(), r.PathValue("shortURL"))
	if err != nil {
		errorJSON(w, status, errorDetail(status))
		return
	}
	writeJSON(w, http.StatusOK, record)
}

// redirect resolves a short URL, queues an analytics event, and redirects the client.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	record, status, err := s.resolve(r.Context(), r.PathValue("shortURL"))
	if err != nil {
		errorJSON(w, status, errorDetail(status))
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func errorJSON(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

// #endregion
