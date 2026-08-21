package inmemory

import (
	"errors"
	"sync"
	"testing"

	"github.com/ksysoev/mimir/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// ---- Get ----

func TestStore_Get_NotFound(t *testing.T) {
	s := NewStore()
	_, err := s.Get(t.Context(), "missing")
	assert.ErrorIs(t, err, core.ErrNotFound)
}

func TestStore_Get_Found(t *testing.T) {
	s := NewStore()
	_, err := s.Put(t.Context(), "k", []byte(`{"a":1}`), "application/json", nil)
	require.NoError(t, err)

	item, err := s.Get(t.Context(), "k")
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Equal(t, "application/json", item.ContentType)
	assert.Greater(t, item.Version, int64(0))
}

// ---- Put ----

func TestStore_Put_NewKey(t *testing.T) {
	s := NewStore()
	item, err := s.Put(t.Context(), "k", []byte(`"hello"`), "text/plain", nil)
	require.NoError(t, err)
	assert.Equal(t, "k", item.Key)
	assert.Equal(t, "text/plain", item.ContentType)
	assert.Greater(t, item.Version, int64(0))
	assert.Equal(t, []byte(`"hello"`), item.Value)
}

func TestStore_Put_DefaultContentType(t *testing.T) {
	s := NewStore()
	item, err := s.Put(t.Context(), "k", []byte(`data`), "", nil)
	require.NoError(t, err)
	assert.Equal(t, core.DefaultContentType, item.ContentType)
}

func TestStore_Put_UpdateKey(t *testing.T) {
	s := NewStore()
	first, err := s.Put(t.Context(), "k", []byte(`1`), "application/json", nil)
	require.NoError(t, err)

	second, err := s.Put(t.Context(), "k", []byte(`2`), "application/json", nil)
	require.NoError(t, err)
	assert.Greater(t, second.Version, first.Version)
	assert.Equal(t, []byte(`2`), second.Value)
}

func TestStore_Put_ConditionalSuccess(t *testing.T) {
	s := NewStore()
	first, err := s.Put(t.Context(), "k", []byte(`1`), "application/json", nil)
	require.NoError(t, err)

	_, err = s.Put(t.Context(), "k", []byte(`2`), "application/json", ptr(first.Version))
	assert.NoError(t, err)
}

func TestStore_Put_ConditionalMismatch(t *testing.T) {
	s := NewStore()
	_, err := s.Put(t.Context(), "k", []byte(`1`), "application/json", nil)
	require.NoError(t, err)

	_, err = s.Put(t.Context(), "k", []byte(`2`), "application/json", ptr(int64(9999)))
	assert.ErrorIs(t, err, core.ErrVersionMismatch)
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

			_, err := s.Put(t.Context(), key, []byte(`1`), "application/json", nil)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
}

func TestStore_ConcurrentSameKey_NoLostUpdates(t *testing.T) {
	// Each goroutine does a conditional put; at most one can succeed per round.
	// We just ensure no races and no panics under -race.
	s := NewStore()

	first, err := s.Put(t.Context(), "k", []byte(`0`), "application/json", nil)
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

			item, err := s.Put(t.Context(), "k", []byte(`1`), "application/json", ptr(v))
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
