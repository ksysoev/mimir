package middleware

import (
	"net/http"

	"github.com/ksysoev/mimir/pkg/core"
)

const (
	// DefaultMaxBodySize is the default maximum request body size in bytes (10 KB).
	DefaultMaxBodySize int64 = 10 * 1024
)

// jsonRequiredMethods contains HTTP methods whose requests must carry an
// application/json body.
var jsonRequiredMethods = map[string]struct{}{
	http.MethodPut:   {},
	http.MethodPatch: {},
}

// NewSanitize returns a middleware that enforces two request-level invariants:
//
//  1. The request body may not exceed maxBodyBytes (default: DefaultMaxBodySize).
//     Oversized bodies are rejected with 413 Request Entity Too Large.
//  2. PUT and PATCH requests must declare Content-Type: application/json.
//     Non-conforming requests are rejected with 415 Unsupported Media Type.
//     The check is MIME-parsed and case-insensitive (consistent with core.Item.IsJSON),
//     so only the exact application/json media type is accepted — application/jsonp
//     and similar are rejected.
//
// A zero or negative maxBodyBytes falls back to DefaultMaxBodySize.
func NewSanitize(maxBodyBytes int64) func(http.Handler) http.Handler {
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodySize
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content-type guard for mutation methods (checked before reading body).
			// Uses core.Item.IsJSON for MIME-aware, case-insensitive matching so that
			// "application/json; charset=utf-8" is accepted but "application/jsonp" is not.
			if _, needsJSON := jsonRequiredMethods[r.Method]; needsJSON {
				if !(core.Item{ContentType: r.Header.Get("Content-Type")}).IsJSON() {
					http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
					return
				}
			}

			// Wrap the body so reads beyond the limit return *http.MaxBytesError.
			// The handler's readBody helper checks for that error and returns 413.
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

			next.ServeHTTP(w, r)
		})
	}
}
