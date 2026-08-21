package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ksysoev/mimir/pkg/repo/kv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// ---- healthCheck ----

func TestAPI_healthCheck_OK(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.On("CheckHealth", mock.Anything).Return(nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	w := httptest.NewRecorder()
	a.healthCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Result().Header.Get("Content-Type"))
	assert.Equal(t, "Ok", w.Body.String())
}

func TestAPI_healthCheck_Error(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.On("CheckHealth", mock.Anything).Return(assert.AnError)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	w := httptest.NewRecorder()
	a.healthCheck(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---- getKey ----

func TestAPI_getKey_Found(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().GetKey(mock.Anything, "user:1").
		Return(kv.Item{Key: "user:1", Value: json.RawMessage(`{"name":"Ari"}`), Version: 3}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/kv/user:1", http.NoBody)
	req.SetPathValue("key", "user:1")

	w := httptest.NewRecorder()
	a.getKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp itemResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "user:1", resp.Key)
	assert.Equal(t, int64(3), resp.Version)
}

func TestAPI_getKey_NotFound(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().GetKey(mock.Anything, "missing").Return(kv.Item{}, kv.ErrNotFound)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/kv/missing", http.NoBody)
	req.SetPathValue("key", "missing")

	w := httptest.NewRecorder()
	a.getKey(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---- putKey ----

func TestAPI_putKey_Success(t *testing.T) {
	mockSvc := NewMockService(t)
	val := json.RawMessage(`{"a":1}`)
	mockSvc.EXPECT().PutKey(mock.Anything, "k", val, (*int64)(nil)).
		Return(kv.Item{Key: "k", Value: val, Version: 1}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp itemResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(1), resp.Version)
}

func TestAPI_putKey_ConditionalSuccess(t *testing.T) {
	mockSvc := NewMockService(t)
	val := json.RawMessage(`{"a":2}`)
	mockSvc.EXPECT().PutKey(mock.Anything, "k", val, ptr(int64(1))).
		Return(kv.Item{Key: "k", Value: val, Version: 2}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPut, "/kv/k?ifVersion=1", strings.NewReader(`{"a":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_putKey_VersionMismatch(t *testing.T) {
	mockSvc := NewMockService(t)
	val := json.RawMessage(`{"a":2}`)
	mockSvc.EXPECT().PutKey(mock.Anything, "k", val, ptr(int64(99))).
		Return(kv.Item{}, kv.ErrVersionMismatch)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPut, "/kv/k?ifVersion=99", strings.NewReader(`{"a":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAPI_putKey_BadJSON(t *testing.T) {
	a := &API{svc: NewMockService(t)}

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`not-json`))
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPI_putKey_BadIfVersion(t *testing.T) {
	a := &API{svc: NewMockService(t)}

	req := httptest.NewRequest(http.MethodPut, "/kv/k?ifVersion=abc", strings.NewReader(`1`))
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- patchKey ----

func TestAPI_patchKey_Success(t *testing.T) {
	mockSvc := NewMockService(t)
	delta := json.RawMessage(`{"b":2}`)
	merged := json.RawMessage(`{"a":1,"b":2}`)
	mockSvc.EXPECT().PatchKey(mock.Anything, "k", delta, (*int64)(nil)).
		Return(kv.Item{Key: "k", Value: merged, Version: 2}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k", strings.NewReader(`{"b":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp itemResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(2), resp.Version)
}

func TestAPI_patchKey_VersionMismatch(t *testing.T) {
	mockSvc := NewMockService(t)
	delta := json.RawMessage(`{}`)
	mockSvc.EXPECT().PatchKey(mock.Anything, "k", delta, ptr(int64(5))).
		Return(kv.Item{}, kv.ErrVersionMismatch)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k?ifVersion=5", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAPI_patchKey_BadJSON(t *testing.T) {
	a := &API{svc: NewMockService(t)}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k", strings.NewReader(`{bad`))
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
