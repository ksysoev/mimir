package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ksysoev/mimir/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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

func TestAPI_getKey_Found_JSON(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().GetKey(mock.Anything, "user:1").
		Return(core.Item{Key: "user:1", Value: []byte(`{"name":"Ari"}`), ContentType: "application/json", Version: 3}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/kv/user:1", http.NoBody)
	req.SetPathValue("key", "user:1")

	w := httptest.NewRecorder()
	a.getKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	assert.Equal(t, "user:1", w.Result().Header.Get(headerKey))
	assert.Equal(t, "3", w.Result().Header.Get(headerVersion))
	assert.Equal(t, `{"name":"Ari"}`, w.Body.String())
}

func TestAPI_getKey_Found_Binary(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().GetKey(mock.Anything, "img").
		Return(core.Item{Key: "img", Value: []byte{0x89, 0x50, 0x4e, 0x47}, ContentType: "image/png", Version: 1}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/kv/img", http.NoBody)
	req.SetPathValue("key", "img")

	w := httptest.NewRecorder()
	a.getKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/png", w.Result().Header.Get("Content-Type"))
	assert.Equal(t, "img", w.Result().Header.Get(headerKey))
	assert.Equal(t, "1", w.Result().Header.Get(headerVersion))
	assert.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47}, w.Body.Bytes())
}

func TestAPI_getKey_NotFound(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().GetKey(mock.Anything, "missing").Return(core.Item{}, core.ErrNotFound)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodGet, "/kv/missing", http.NoBody)
	req.SetPathValue("key", "missing")

	w := httptest.NewRecorder()
	a.getKey(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---- putKey ----

func TestAPI_putKey_JSON(t *testing.T) {
	mockSvc := NewMockService(t)
	val := []byte(`{"a":1}`)
	mockSvc.EXPECT().PutKey(mock.Anything, core.Item{Key: "k", Value: val, ContentType: "application/json", Version: 0}).
		Return(core.Item{Key: "k", Value: val, ContentType: "application/json", Version: 1}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	assert.Equal(t, "k", w.Result().Header.Get(headerKey))
	assert.Equal(t, "1", w.Result().Header.Get(headerVersion))
}

func TestAPI_putKey_PlainText(t *testing.T) {
	mockSvc := NewMockService(t)
	val := []byte("hello world")
	mockSvc.EXPECT().PutKey(mock.Anything, core.Item{Key: "k", Value: val, ContentType: "text/plain", Version: 0}).
		Return(core.Item{Key: "k", Value: val, ContentType: "text/plain", Version: 1}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader("hello world"))
	req.Header.Set("Content-Type", "text/plain")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Result().Header.Get("Content-Type"))
	assert.Equal(t, "hello world", w.Body.String())
}

func TestAPI_putKey_NoContentType_DefaultsToOctetStream(t *testing.T) {
	mockSvc := NewMockService(t)
	val := []byte("raw bytes")
	mockSvc.EXPECT().PutKey(mock.Anything, core.Item{Key: "k", Value: val, ContentType: core.DefaultContentType, Version: 0}).
		Return(core.Item{Key: "k", Value: val, ContentType: core.DefaultContentType, Version: 1}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPut, "/kv/k", strings.NewReader("raw bytes"))
	// intentionally no Content-Type header
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, core.DefaultContentType, w.Result().Header.Get("Content-Type"))
}

func TestAPI_putKey_ConditionalSuccess(t *testing.T) {
	mockSvc := NewMockService(t)
	val := []byte(`{"a":2}`)
	mockSvc.EXPECT().PutKey(mock.Anything, core.Item{Key: "k", Value: val, ContentType: "application/json", Version: 1}).
		Return(core.Item{Key: "k", Value: val, ContentType: "application/json", Version: 2}, nil)
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
	val := []byte(`{"a":2}`)
	mockSvc.EXPECT().PutKey(mock.Anything, core.Item{Key: "k", Value: val, ContentType: "application/json", Version: 99}).
		Return(core.Item{}, core.ErrVersionMismatch)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPut, "/kv/k?ifVersion=99", strings.NewReader(`{"a":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.putKey(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
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
	delta := []byte(`{"b":2}`)
	merged := []byte(`{"a":1,"b":2}`)
	mockSvc.EXPECT().PatchKey(mock.Anything, core.Item{Key: "k", Value: delta, ContentType: "application/json", Version: 0}).
		Return(core.Item{Key: "k", Value: merged, ContentType: "application/json", Version: 2}, nil)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k", strings.NewReader(`{"b":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	assert.Equal(t, "k", w.Result().Header.Get(headerKey))
	assert.Equal(t, "2", w.Result().Header.Get(headerVersion))
}

func TestAPI_patchKey_NonJSONContentType_Returns415(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().PatchKey(mock.Anything, core.Item{Key: "k", Value: []byte(`raw`), ContentType: "text/plain", Version: 0}).
		Return(core.Item{}, core.ErrUnsupportedContentType)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k", strings.NewReader(`raw`))
	req.Header.Set("Content-Type", "text/plain")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestAPI_patchKey_MissingContentType_Returns415(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().PatchKey(mock.Anything, core.Item{Key: "k", Value: []byte(`{}`), ContentType: core.DefaultContentType, Version: 0}).
		Return(core.Item{}, core.ErrUnsupportedContentType)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k", strings.NewReader(`{}`))
	// intentionally no Content-Type header
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestAPI_patchKey_InvalidJSON_Returns400(t *testing.T) {
	mockSvc := NewMockService(t)
	mockSvc.EXPECT().PatchKey(mock.Anything, core.Item{Key: "k", Value: []byte(`{bad`), ContentType: "application/json", Version: 0}).
		Return(core.Item{}, core.ErrInvalidPayload)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPI_patchKey_VersionMismatch(t *testing.T) {
	mockSvc := NewMockService(t)
	delta := []byte(`{}`)
	mockSvc.EXPECT().PatchKey(mock.Anything, core.Item{Key: "k", Value: delta, ContentType: "application/json", Version: 5}).
		Return(core.Item{}, core.ErrVersionMismatch)
	a := &API{svc: mockSvc}

	req := httptest.NewRequest(http.MethodPatch, "/kv/k?ifVersion=5", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	a.patchKey(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// ---- readBody helper ----

func TestReadBody_DefaultContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("data"))
	// no Content-Type set
	w := httptest.NewRecorder()

	body, ct, ok := readBody(w, req)
	require.True(t, ok)
	assert.Equal(t, []byte("data"), body)
	assert.Equal(t, core.DefaultContentType, ct)
}

func TestReadBody_PreservesContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	_, ct, ok := readBody(w, req)
	require.True(t, ok)
	assert.Equal(t, "application/json", ct)
}
