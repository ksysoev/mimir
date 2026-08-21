package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestNew(t *testing.T) {
	store := NewMockkvStore(t)
	svc := New(store)
	assert.NotNil(t, svc)
}

func TestService_CheckHealth(t *testing.T) {
	store := NewMockkvStore(t)
	svc := New(store)
	assert.NoError(t, svc.CheckHealth(t.Context()))
}

func TestService_GetKey_Found(t *testing.T) {
	store := NewMockkvStore(t)
	store.EXPECT().Get("k").Return(Item{Key: "k", Value: []byte(`1`), ContentType: "application/json", Version: 1}, nil)

	svc := New(store)
	item, err := svc.GetKey(t.Context(), "k")
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Equal(t, "application/json", item.ContentType)
}

func TestService_GetKey_NotFound(t *testing.T) {
	store := NewMockkvStore(t)
	store.EXPECT().Get("missing").Return(Item{}, ErrNotFound)

	svc := New(store)
	_, err := svc.GetKey(t.Context(), "missing")
	assert.Error(t, err)
}

func TestService_PutKey_Success(t *testing.T) {
	store := NewMockkvStore(t)
	val := []byte(`"v"`)
	store.EXPECT().Put("k", val, "text/plain", (*int64)(nil)).
		Return(Item{Key: "k", Value: val, ContentType: "text/plain", Version: 1}, nil)

	svc := New(store)
	item, err := svc.PutKey(t.Context(), "k", val, "text/plain", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), item.Version)
	assert.Equal(t, "text/plain", item.ContentType)
}

func TestService_PutKey_VersionMismatch(t *testing.T) {
	store := NewMockkvStore(t)
	val := []byte(`"v"`)
	store.EXPECT().Put("k", val, "application/json", ptr(int64(99))).
		Return(Item{}, ErrVersionMismatch)

	svc := New(store)
	_, err := svc.PutKey(t.Context(), "k", val, "application/json", ptr(int64(99)))
	assert.ErrorIs(t, err, ErrVersionMismatch)
}

func TestService_PatchKey_Success(t *testing.T) {
	store := NewMockkvStore(t)
	delta := []byte(`{"a":1}`)
	store.EXPECT().Patch("k", delta, (*int64)(nil)).
		Return(Item{Key: "k", Value: delta, ContentType: "application/json", Version: 2}, nil)

	svc := New(store)
	item, err := svc.PatchKey(t.Context(), "k", delta, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), item.Version)
	assert.Equal(t, "application/json", item.ContentType)
}

func TestService_PatchKey_VersionMismatch(t *testing.T) {
	store := NewMockkvStore(t)
	delta := []byte(`{}`)
	store.EXPECT().Patch("k", delta, ptr(int64(5))).Return(Item{}, ErrVersionMismatch)

	svc := New(store)
	_, err := svc.PatchKey(t.Context(), "k", delta, ptr(int64(5)))
	assert.ErrorIs(t, err, ErrVersionMismatch)
}
