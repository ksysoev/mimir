package middleware

import (
	"crypto/subtle"
	"net/http"
)

const (
	// apiKeyHeader is the request header that carries the API key.
	apiKeyHeader = "X-API-Key" //nolint:gosec // this is a header name, not a credential
)

// NewAPIKey returns a middleware that validates the X-API-Key request header
// against the provided static key using constant-time comparison to prevent
// timing-based key enumeration attacks.
//
// When apiKey is empty the middleware is disabled and every request passes
// through unchanged — this allows the feature to be opted into via config.
//
// Requests that are missing the header or supply a wrong value receive
// 401 Unauthorized.
func NewAPIKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Fast-path: auth disabled when no key is configured.
		if apiKey == "" {
			return next
		}

		expected := []byte(apiKey)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get(apiKeyHeader))

			if subtle.ConstantTimeCompare(got, expected) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
