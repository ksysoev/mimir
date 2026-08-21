package core

import (
	"encoding/json"
	"testing"

	"github.com/ksysoev/mimir/pkg/repo/kv"
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
	store.EXPECT().Get("k").Return(kv.Item{Key: "k", Value: json.RawMessage(`1`), Version: 1}, nil)

	svc := New(store)
	item, err := svc.GetKey(t.Context(), "k")
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
}

func TestService_GetKey_NotFound(t *testing.T) {
	store := NewMockkvStore(t)
	store.EXPECT().Get("missing").Return(kv.Item{}, kv.ErrNotFound)

	svc := New(store)
	_, err := svc.GetKey(t.Context(), "missing")
	assert.Error(t, err)
}

func TestService_PutKey_Success(t *testing.T) {
	store := NewMockkvStore(t)
	val := json.RawMessage(`"v"`)
	store.EXPECT().Put("k", val, (*int64)(nil)).Return(kv.Item{Key: "k", Value: val, Version: 1}, nil)

	svc := New(store)
	item, err := svc.PutKey(t.Context(), "k", val, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), item.Version)
}

func TestService_PutKey_VersionMismatch(t *testing.T) {
	store := NewMockkvStore(t)
	val := json.RawMessage(`"v"`)
	store.EXPECT().Put("k", val, ptr(int64(99))).Return(kv.Item{}, kv.ErrVersionMismatch)

	svc := New(store)
	_, err := svc.PutKey(t.Context(), "k", val, ptr(int64(99)))
	assert.ErrorIs(t, err, kv.ErrVersionMismatch)
}

func TestService_PatchKey_Success(t *testing.T) {
	store := NewMockkvStore(t)
	delta := json.RawMessage(`{"a":1}`)
	store.EXPECT().Patch("k", delta, (*int64)(nil)).Return(kv.Item{Key: "k", Value: delta, Version: 2}, nil)

	svc := New(store)
	item, err := svc.PatchKey(t.Context(), "k", delta, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), item.Version)
}

func TestService_PatchKey_VersionMismatch(t *testing.T) {
	store := NewMockkvStore(t)
	delta := json.RawMessage(`{}`)
	store.EXPECT().Patch("k", delta, ptr(int64(5))).Return(kv.Item{}, kv.ErrVersionMismatch)

	svc := New(store)
	_, err := svc.PatchKey(t.Context(), "k", delta, ptr(int64(5)))
	assert.ErrorIs(t, err, kv.ErrVersionMismatch)
}
