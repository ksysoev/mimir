package kv

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// ---- Get ----

func TestStore_Get_NotFound(t *testing.T) {
	s := NewStore()
	_, err := s.Get("missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestStore_Get_Found(t *testing.T) {
	s := NewStore()
	_, err := s.Put("k", json.RawMessage(`{"a":1}`), nil)
	require.NoError(t, err)

	item, err := s.Get("k")
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Greater(t, item.Version, int64(0))
}

// ---- Put ----

func TestStore_Put_NewKey(t *testing.T) {
	s := NewStore()
	item, err := s.Put("k", json.RawMessage(`"hello"`), nil)
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Greater(t, item.Version, int64(0))
	assert.Equal(t, json.RawMessage(`"hello"`), item.Value)
}

func TestStore_Put_UpdateKey(t *testing.T) {
	s := NewStore()
	first, err := s.Put("k", json.RawMessage(`1`), nil)
	require.NoError(t, err)

	second, err := s.Put("k", json.RawMessage(`2`), nil)
	require.NoError(t, err)
	assert.Greater(t, second.Version, first.Version)
	assert.Equal(t, json.RawMessage(`2`), second.Value)
}

func TestStore_Put_ConditionalSuccess(t *testing.T) {
	s := NewStore()
	first, err := s.Put("k", json.RawMessage(`1`), nil)
	require.NoError(t, err)

	_, err = s.Put("k", json.RawMessage(`2`), ptr(first.Version))
	assert.NoError(t, err)
}

func TestStore_Put_ConditionalMismatch(t *testing.T) {
	s := NewStore()
	_, err := s.Put("k", json.RawMessage(`1`), nil)
	require.NoError(t, err)

	_, err = s.Put("k", json.RawMessage(`2`), ptr(int64(9999)))
	assert.ErrorIs(t, err, ErrVersionMismatch)
}

// ---- Patch ----

func TestStore_Patch_CreateNew(t *testing.T) {
	s := NewStore()
	item, err := s.Patch("k", json.RawMessage(`{"x":1}`), nil)
	require.NoError(t, err)
	assert.Greater(t, item.Version, int64(0))

	var m map[string]int
	require.NoError(t, json.Unmarshal(item.Value, &m))
	assert.Equal(t, 1, m["x"])
}

func TestStore_Patch_MergeObjects(t *testing.T) {
	s := NewStore()
	_, err := s.Put("k", json.RawMessage(`{"a":1,"b":2}`), nil)
	require.NoError(t, err)

	item, err := s.Patch("k", json.RawMessage(`{"b":99,"c":3}`), nil)
	require.NoError(t, err)

	var m map[string]int
	require.NoError(t, json.Unmarshal(item.Value, &m))
	assert.Equal(t, 1, m["a"])
	assert.Equal(t, 99, m["b"])
	assert.Equal(t, 3, m["c"])
}

func TestStore_Patch_ReplaceNonObject(t *testing.T) {
	s := NewStore()
	_, err := s.Put("k", json.RawMessage(`42`), nil)
	require.NoError(t, err)

	item, err := s.Patch("k", json.RawMessage(`"replaced"`), nil)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`"replaced"`), item.Value)
}

func TestStore_Patch_ConditionalMismatch(t *testing.T) {
	s := NewStore()
	_, err := s.Put("k", json.RawMessage(`{}`), nil)
	require.NoError(t, err)

	_, err = s.Patch("k", json.RawMessage(`{"x":1}`), ptr(int64(9999)))
	assert.ErrorIs(t, err, ErrVersionMismatch)
}

func TestStore_Patch_VersionIncrements(t *testing.T) {
	s := NewStore()
	first, err := s.Put("k", json.RawMessage(`{"a":1}`), nil)
	require.NoError(t, err)

	second, err := s.Patch("k", json.RawMessage(`{"b":2}`), nil)
	require.NoError(t, err)
	assert.Greater(t, second.Version, first.Version)
}

// ---- Concurrency ----

func TestStore_ConcurrentDifferentKeys(t *testing.T) {
	s := NewStore()

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()

			key := "key"

			if n%2 == 0 {
				key = "other"
			}

			_, err := s.Put(key, json.RawMessage(`1`), nil)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
}

func TestStore_ConcurrentSameKey_NoLostUpdates(t *testing.T) {
	// Each goroutine does a conditional put; at most one can succeed per round.
	// We just ensure no races and no panics under -race.
	s := NewStore()

	first, err := s.Put("k", json.RawMessage(`0`), nil)
	require.NoError(t, err)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes int
	)

	currentVersion := first.Version

	for range 50 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			mu.Lock()
			v := currentVersion
			mu.Unlock()

			item, err := s.Put("k", json.RawMessage(`1`), ptr(v))
			if err == nil {
				mu.Lock()
				currentVersion = item.Version
				successes++
				mu.Unlock()
			} else {
				assert.True(t, errors.Is(err, ErrVersionMismatch))
			}
		}()
	}

	wg.Wait()
	assert.Greater(t, successes, 0)
}

// ---- Helpers ----

func TestIsJSONObject(t *testing.T) {
	assert.True(t, isJSONObject(json.RawMessage(`{}`)))
	assert.True(t, isJSONObject(json.RawMessage(`  { "a": 1 }`)))
	assert.False(t, isJSONObject(json.RawMessage(`[]`)))
	assert.False(t, isJSONObject(json.RawMessage(`"str"`)))
	assert.False(t, isJSONObject(json.RawMessage(`42`)))
	assert.False(t, isJSONObject(json.RawMessage(``)))
}

func TestShallowMerge(t *testing.T) {
	base := json.RawMessage(`{"a":1,"b":2}`)
	delta := json.RawMessage(`{"b":99,"c":3}`)

	result, err := shallowMerge(base, delta)
	require.NoError(t, err)

	var m map[string]int
	require.NoError(t, json.Unmarshal(result, &m))
	assert.Equal(t, 1, m["a"])
	assert.Equal(t, 99, m["b"])
	assert.Equal(t, 3, m["c"])
}
