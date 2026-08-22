package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// okHandler is a trivial handler that always returns 200.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestNewSanitize_DefaultMaxBodySize(t *testing.T) {
	// Passing 0 should use DefaultMaxBodySize without panicking.
	mw := NewSanitize(0)
	assert.NotNil(t, mw)
}

// ---- Content-Type enforcement ----

func TestNewSanitize_PUT_JSONContentType_PassesThrough(t *testing.T) {
	h := NewSanitize(DefaultMaxBodySize)(okHandler)

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewSanitize_PUT_NoContentType_Returns415(t *testing.T) {
	h := NewSanitize(DefaultMaxBodySize)(okHandler)

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader("data"))
	// no Content-Type header
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestNewSanitize_PUT_WrongContentType_Returns415(t *testing.T) {
	h := NewSanitize(DefaultMaxBodySize)(okHandler)

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader("data"))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestNewSanitize_PATCH_NonJSONContentType_Returns415(t *testing.T) {
	h := NewSanitize(DefaultMaxBodySize)(okHandler)

	req := httptest.NewRequest(http.MethodPatch, "/kv/k", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/xml")

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestNewSanitize_PUT_JSONPContentType_Returns415(t *testing.T) {
	// application/jsonp shares the "application/json" prefix but must be rejected.
	// This verifies that MIME-parsed matching is used, not a plain HasPrefix check.
	h := NewSanitize(DefaultMaxBodySize)(okHandler)

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/jsonp")

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestNewSanitize_GET_NoContentTypeRestriction(t *testing.T) {
	h := NewSanitize(DefaultMaxBodySize)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/kv/k", http.NoBody)
	// GET has no content-type requirement
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- Body size enforcement ----

func TestNewSanitize_BodyWithinLimit_PassesThrough(t *testing.T) {
	const limit = 100

	h := NewSanitize(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Consume the body inside the handler to trigger the size check.
		buf := make([]byte, limit)
		_, _ = r.Body.Read(buf)

		w.WriteHeader(http.StatusOK)
	}))

	// "x…x" with (limit-2) chars wrapped in JSON quotes = exactly limit bytes.
	body := strings.Repeat("x", limit-2)
	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`"`+body+`"`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewSanitize_JSONContentTypeWithCharset_PassesThrough(t *testing.T) {
	// application/json; charset=utf-8 must be accepted.
	h := NewSanitize(DefaultMaxBodySize)(okHandler)

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
