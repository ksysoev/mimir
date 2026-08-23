package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAPIKey_Disabled_WhenKeyEmpty(t *testing.T) {
	// Empty key → middleware is a no-op; all requests pass through.
	h := NewAPIKey("")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/kv/k", http.NoBody)
	// No X-API-Key header intentionally.
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewAPIKey_ValidKey_PassesThrough(t *testing.T) {
	const key = "super-secret"

	h := NewAPIKey(key)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/kv/k", http.NoBody)
	req.Header.Set(apiKeyHeader, key)

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewAPIKey_MissingHeader_Returns401(t *testing.T) {
	h := NewAPIKey("secret")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/kv/k", http.NoBody)

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNewAPIKey_WrongKey_Returns401(t *testing.T) {
	h := NewAPIKey("correct")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/kv/k", http.NoBody)
	req.Header.Set(apiKeyHeader, "wrong")

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
