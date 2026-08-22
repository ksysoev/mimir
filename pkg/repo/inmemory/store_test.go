package inmemory

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/ksysoev/mimir/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Get_NotFound(t *testing.T) {
	s := NewStore(Config{})
	_, err := s.Get(t.Context(), "missing")
	assert.ErrorIs(t, err, core.ErrNotFound)
}

func TestStore_Get_Found(t *testing.T) {
	s := NewStore(Config{})
	_, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`{"a":1}`), ContentType: "application/json"})
	require.NoError(t, err)

	item, err := s.Get(t.Context(), "k")
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Equal(t, "application/json", item.ContentType)
	assert.Greater(t, item.Version, uint64(0))
}

// ---- Put ----

func TestStore_Put_NewKey(t *testing.T) {
	s := NewStore(Config{})
	item, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`"hello"`), ContentType: "text/plain"})
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Equal(t, "text/plain", item.ContentType)
	assert.Greater(t, item.Version, uint64(0))
	assert.Equal(t, []byte(`"hello"`), item.Value)
}

func TestStore_Put_DefaultContentType(t *testing.T) {
	s := NewStore(Config{})
	item, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`data`)})
	require.NoError(t, err)
	assert.Equal(t, core.DefaultContentType, item.ContentType)
}

func TestStore_Put_UpdateKey(t *testing.T) {
	s := NewStore(Config{})
	first, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`1`), ContentType: "application/json"})
	require.NoError(t, err)

	second, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`2`), ContentType: "application/json"})
	require.NoError(t, err)
	assert.Greater(t, second.Version, first.Version)
	assert.Equal(t, []byte(`2`), second.Value)
}

func TestStore_Put_ConditionalSuccess(t *testing.T) {
	s := NewStore(Config{})
	first, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`1`), ContentType: "application/json"})
	require.NoError(t, err)

	_, err = s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`2`), ContentType: "application/json", Version: first.Version})
	assert.NoError(t, err)
}

func TestStore_Put_ConditionalMismatch(t *testing.T) {
	s := NewStore(Config{})
	_, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`1`), ContentType: "application/json"})
	require.NoError(t, err)

	_, err = s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`2`), ContentType: "application/json", Version: 9999})
	assert.ErrorIs(t, err, core.ErrVersionMismatch)
}

// ---- MaxKeys / capacity ----

func TestStore_DefaultMaxKeys(t *testing.T) {
	s := NewStore(Config{})
	assert.Equal(t, DefaultMaxKeys, s.maxKeys)
}

func TestStore_MaxKeys_RejectsNewKeyWhenFull(t *testing.T) {
	const limit = 3

	s := NewStore(Config{MaxKeys: limit})

	for i := range limit {
		_, err := s.Put(t.Context(), core.Item{Key: fmt.Sprintf("key%d", i), Value: []byte(`1`), ContentType: "application/json"})
		require.NoError(t, err)
	}

	// One more distinct key must fail.
	_, err := s.Put(t.Context(), core.Item{Key: "overflow", Value: []byte(`1`), ContentType: "application/json"})
	assert.ErrorIs(t, err, core.ErrStoreFull)
}

func TestStore_MaxKeys_AllowsUpdatesWhenFull(t *testing.T) {
	const limit = 2

	s := NewStore(Config{MaxKeys: limit})

	for i := range limit {
		_, err := s.Put(t.Context(), core.Item{Key: fmt.Sprintf("key%d", i), Value: []byte(`1`), ContentType: "application/json"})
		require.NoError(t, err)
	}

	// Updating an existing key must still succeed even when the store is at capacity.
	_, err := s.Put(t.Context(), core.Item{Key: "key0", Value: []byte(`2`), ContentType: "application/json"})
	assert.NoError(t, err)
}

// ---- Concurrency ----

func TestStore_ConcurrentDifferentKeys(t *testing.T) {
	s := NewStore(Config{})

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()

			key := "key"
			if n%2 == 0 {
				key = "other"
			}

			_, err := s.Put(t.Context(), core.Item{Key: key, Value: []byte(`1`), ContentType: "application/json"})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
}

func TestStore_ConcurrentSameKey_NoLostUpdates(t *testing.T) {
	// Each goroutine does a conditional put; at most one can succeed per round.
	// We just ensure no races and no panics under -race.
	s := NewStore(Config{})

	first, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`0`), ContentType: "application/json"})
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

			item, err := s.Put(t.Context(), core.Item{Key: "k", Value: []byte(`1`), ContentType: "application/json", Version: v})
			if err == nil {
				mu.Lock()
				currentVersion = item.Version
				successes++
				mu.Unlock()
			} else {
				assert.True(t, errors.Is(err, core.ErrVersionMismatch))
			}
		}()
	}

	wg.Wait()
	assert.Greater(t, successes, 0)
}
