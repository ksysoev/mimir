package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
	store.EXPECT().Get(mock.Anything, "k").Return(Item{Key: "k", Value: []byte(`1`), ContentType: "application/json", Version: 1}, nil)

	svc := New(store)
	item, err := svc.GetKey(t.Context(), "k")
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Equal(t, "application/json", item.ContentType)
}

func TestService_GetKey_NotFound(t *testing.T) {
	store := NewMockkvStore(t)
	store.EXPECT().Get(mock.Anything, "missing").Return(Item{}, ErrNotFound)

	svc := New(store)
	_, err := svc.GetKey(t.Context(), "missing")
	assert.Error(t, err)
}

func TestService_PutKey_Success(t *testing.T) {
	store := NewMockkvStore(t)
	val := []byte(`"v"`)
	store.EXPECT().Put(mock.Anything, Item{Key: "k", Value: val, ContentType: "text/plain", Version: 0}).
		Return(Item{Key: "k", Value: val, ContentType: "text/plain", Version: 1}, nil)

	svc := New(store)
	item, err := svc.PutKey(t.Context(), Item{Key: "k", Value: val, ContentType: "text/plain"})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), item.Version)
	assert.Equal(t, "text/plain", item.ContentType)
}

func TestService_PutKey_VersionMismatch(t *testing.T) {
	store := NewMockkvStore(t)
	val := []byte(`"v"`)
	store.EXPECT().Put(mock.Anything, Item{Key: "k", Value: val, ContentType: "application/json", Version: 99}).
		Return(Item{}, ErrVersionMismatch)

	svc := New(store)
	_, err := svc.PutKey(t.Context(), Item{Key: "k", Value: val, ContentType: "application/json", Version: 99})
	assert.ErrorIs(t, err, ErrVersionMismatch)
}

// ---- PatchKey ----

func TestService_PatchKey_NewKey(t *testing.T) {
	store := NewMockkvStore(t)
	delta := []byte(`{"a":1}`)

	// key does not exist yet; Version == 0 means unconditional write
	store.EXPECT().Get(mock.Anything, "k").Return(Item{}, ErrNotFound)
	store.EXPECT().Put(mock.Anything, Item{Key: "k", Value: delta, ContentType: "application/json", Version: 0}).
		Return(Item{Key: "k", Value: delta, ContentType: "application/json", Version: 1}, nil)

	svc := New(store)
	item, err := svc.PatchKey(t.Context(), Item{Key: "k", Value: delta, ContentType: "application/json"})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), item.Version)
	assert.Equal(t, "application/json", item.ContentType)
}

func TestService_PatchKey_MergeObjects(t *testing.T) {
	store := NewMockkvStore(t)
	existing := Item{Key: "k", Value: []byte(`{"a":1,"b":2}`), ContentType: "application/json", Version: 3}
	merged := []byte(`{"a":1,"b":99,"c":3}`)

	store.EXPECT().Get(mock.Anything, "k").Return(existing, nil)
	store.EXPECT().Put(mock.Anything, Item{Key: "k", Value: merged, ContentType: "application/json", Version: 3}).
		Return(Item{Key: "k", Value: merged, ContentType: "application/json", Version: 4}, nil)

	svc := New(store)
	// Version == 0 in the request means no caller-side guard
	item, err := svc.PatchKey(t.Context(), Item{Key: "k", Value: []byte(`{"b":99,"c":3}`), ContentType: "application/json"})
	require.NoError(t, err)
	assert.Equal(t, uint64(4), item.Version)
}

func TestService_PatchKey_ReplaceNonObject(t *testing.T) {
	store := NewMockkvStore(t)
	existing := Item{Key: "k", Value: []byte(`42`), ContentType: "application/json", Version: 2}
	delta := []byte(`"replaced"`)

	store.EXPECT().Get(mock.Anything, "k").Return(existing, nil)
	store.EXPECT().Put(mock.Anything, Item{Key: "k", Value: delta, ContentType: "application/json", Version: 2}).
		Return(Item{Key: "k", Value: delta, ContentType: "application/json", Version: 3}, nil)

	svc := New(store)
	item, err := svc.PatchKey(t.Context(), Item{Key: "k", Value: delta, ContentType: "application/json"})
	require.NoError(t, err)
	assert.Equal(t, uint64(3), item.Version)
}

func TestService_PatchKey_CallerVersionMismatch(t *testing.T) {
	store := NewMockkvStore(t)
	existing := Item{Key: "k", Value: []byte(`{}`), ContentType: "application/json", Version: 2}

	store.EXPECT().Get(mock.Anything, "k").Return(existing, nil)
	// Put must NOT be called — mismatch is detected in core before writing

	svc := New(store)
	// Version == 99 acts as a CAS guard; existing is at 2 → mismatch
	_, err := svc.PatchKey(t.Context(), Item{Key: "k", Value: []byte(`{"x":1}`), ContentType: "application/json", Version: 99})
	assert.ErrorIs(t, err, ErrVersionMismatch)
}

func TestService_PatchKey_ConcurrentWriteMismatch(t *testing.T) {
	store := NewMockkvStore(t)
	existing := Item{Key: "k", Value: []byte(`{"a":1}`), ContentType: "application/json", Version: 5}
	delta := []byte(`{"b":2}`)

	store.EXPECT().Get(mock.Anything, "k").Return(existing, nil)
	// concurrent write between Get and Put causes the CAS to fail
	store.EXPECT().Put(mock.Anything, Item{Key: "k", Value: []byte(`{"a":1,"b":2}`), ContentType: "application/json", Version: 5}).
		Return(Item{}, ErrVersionMismatch)

	svc := New(store)
	_, err := svc.PatchKey(t.Context(), Item{Key: "k", Value: delta, ContentType: "application/json"})
	assert.ErrorIs(t, err, ErrVersionMismatch)
}

func TestService_PatchKey_NonJSONContentType(t *testing.T) {
	store := NewMockkvStore(t)
	svc := New(store)
	// store must NOT be called — validation fails before any I/O
	_, err := svc.PatchKey(t.Context(), Item{Key: "k", Value: []byte(`{"a":1}`), ContentType: "text/plain"})
	assert.ErrorIs(t, err, ErrUnsupportedContentType)
}

func TestService_PatchKey_InvalidJSON(t *testing.T) {
	store := NewMockkvStore(t)
	svc := New(store)
	// store must NOT be called — validation fails before any I/O
	_, err := svc.PatchKey(t.Context(), Item{Key: "k", Value: []byte(`{bad json`), ContentType: "application/json"})
	assert.ErrorIs(t, err, ErrInvalidPayload)
}
