package shortener

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const maxURLLength = 2083

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ready(ctx); err != nil {
		slog.Warn("DynamoDB readiness check failed", "error", err)
		errorJSON(w, http.StatusServiceUnavailable, "Data store unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) shorten(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LongURL string `json:"long_url"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxURLLength+256))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || !validHTTPURL(body.LongURL) {
		errorJSON(w, http.StatusUnprocessableEntity, "Invalid long_url")
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		errorJSON(w, http.StatusUnprocessableEntity, "Invalid request body")
		return
	}
	record, err := s.store.Create(r.Context(), body.LongURL)
	if err != nil {
		slog.Error("URL creation failed", "error", err)
		errorJSON(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) lookup(w http.ResponseWriter, r *http.Request) {
	record, status, err := s.resolve(r.Context(), r.PathValue("shortURL"))
	if err != nil {
		errorJSON(w, status, errorDetail(status))
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	record, status, err := s.resolve(r.Context(), r.PathValue("shortURL"))
	if err != nil {
		errorJSON(w, status, errorDetail(status))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()
	if err := s.publisher.PublishRedirect(ctx, record.ShortURL); err != nil {
		slog.Warn("Redirect analytics enqueue failed", "short_url", record.ShortURL, "error", err)
	}
	http.Redirect(w, r, record.LongURL, http.StatusFound)
}

func (s *Server) resolve(ctx context.Context, value string) (URLRecord, int, error) {
	if !validShortURL(value) {
		return URLRecord{}, http.StatusUnprocessableEntity, errors.New("invalid short URL")
	}
	record, err := s.store.Get(ctx, value)
	if errors.Is(err, ErrNotFound) {
		return URLRecord{}, http.StatusNotFound, err
	}
	if err != nil {
		return URLRecord{}, http.StatusInternalServerError, err
	}
	return record, 0, nil
}

func validHTTPURL(raw string) bool {
	if len(raw) == 0 || len(raw) > maxURLLength {
		return false
	}
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https")
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func errorDetail(status int) string {
	switch status {
	case http.StatusNotFound:
		return "Short URL not found"
	case http.StatusUnprocessableEntity:
		return "Invalid short URL"
	default:
		return "Internal server error"
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func errorJSON(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}
