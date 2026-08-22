package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ksysoev/mimir/pkg/core"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAPI_newMux_LivezRoute(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().CheckHealth(mock.Anything).Return(nil)

	a, err := New(Config{Listen: ":0"}, mockSvc)
	require.NoError(t, err)

	mux := a.newMux()

	req := httptest.NewRequest("GET", "/livez", http.NoBody)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "expected status 200")

	body := w.Body.String()
	assert.Equal(t, "Ok", body)
}

// ---- /kv middleware integration ----

func TestAPI_newMux_KV_MissingAPIKey_Returns401(t *testing.T) {
	a, err := New(Config{Listen: ":0", Key: "secret"}, NewMockService(t))
	require.NoError(t, err)

	mux := a.newMux()

	// Valid request in every way except the missing auth header.
	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPI_newMux_KV_WrongAPIKey_Returns401(t *testing.T) {
	a, err := New(Config{Listen: ":0", Key: "secret"}, NewMockService(t))
	require.NoError(t, err)

	mux := a.newMux()

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "wrong")

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPI_newMux_KV_AuthPrecedesSanitize_BadCTWithNoKey_Returns401(t *testing.T) {
	// Auth runs before sanitize: even a bad Content-Type must yield 401, not 415,
	// when the API key is missing. This verifies the middleware order.
	a, err := New(Config{Listen: ":0", Key: "secret"}, NewMockService(t))
	require.NoError(t, err)

	mux := a.newMux()

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader("data"))
	req.Header.Set("Content-Type", "text/plain") // wrong CT, but auth fires first
	// no X-API-Key header

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAPI_newMux_KV_WrongContentType_Returns415(t *testing.T) {
	a, err := New(Config{Listen: ":0"}, NewMockService(t)) // no API key
	require.NoError(t, err)

	mux := a.newMux()

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader("data"))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestAPI_newMux_KV_BodyTooLarge_Returns413(t *testing.T) {
	const limit = 10

	a, err := New(Config{Listen: ":0", MaxBodySize: limit}, NewMockService(t))
	require.NoError(t, err)

	mux := a.newMux()

	// Body is well over the 10-byte limit.
	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestAPI_newMux_KV_StoreFull_Returns507(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().PutKey(mock.Anything, mock.Anything).Return(core.Item{}, core.ErrStoreFull)

	a, err := New(Config{Listen: ":0"}, mockSvc)
	require.NoError(t, err)

	mux := a.newMux()

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInsufficientStorage, w.Code)
}

func TestAPI_newMux_ListKeys_Route(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().ListKeys(mock.Anything, "").Return([]core.KeyEntry{})

	a, err := New(Config{Listen: ":0"}, mockSvc)
	require.NoError(t, err)

	mux := a.newMux()

	req := httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-ndjson", w.Result().Header.Get("Content-Type"))
}

func TestAPI_newMux_ListKeys_RequiresAuth(t *testing.T) {
	a, err := New(Config{Listen: ":0", Key: "secret"}, NewMockService(t))
	require.NoError(t, err)

	mux := a.newMux()

	req := httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
