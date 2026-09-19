package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const maxBodyBytes = 64 << 10

// Decode reads exactly one JSON value from r into dst.
//
// The request must use the application/json media type and its body must not
// exceed maxBodyBytes. Unknown object fields, an empty body, malformed JSON,
// and any value following the first JSON value are rejected. The returned error
// is suitable for deciding that the request is invalid; Decode does not write an
// error response itself.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}

// Write sends value as a JSON response with the supplied HTTP status code.
func Write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// Error sends a JSON error response containing detail and the supplied HTTP status code.
func Error(w http.ResponseWriter, status int, detail string) {
	Write(w, status, map[string]string{"detail": detail})
}
