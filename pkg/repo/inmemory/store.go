// Package inmemory provides an in-memory key-value store with per-key atomicity and versioning.
package inmemory

import (
	"context"
	"sync"

	"github.com/ksysoev/mimir/pkg/core"
)

// entry is the internal per-key structure. It carries its own mutex so that
// concurrent operations on different keys do not block each other.
type entry struct {
	contentType string
	value       []byte
	version     uint64
	mu          sync.Mutex
}

// Store is an in-memory key-value store. The map-level RWMutex guards the map
// structure itself; the per-entry mutex serialises operations on a single key.
type Store struct {
	data map[string]*entry
	mu   sync.RWMutex
}

// NewStore creates and returns an empty Store.
func NewStore() *Store {
	return &Store{data: make(map[string]*entry)}
}

// Get returns the Item for key. Returns core.ErrNotFound if the key does not exist.
func (s *Store) Get(_ context.Context, key string) (core.Item, error) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return core.Item{}, core.ErrNotFound
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return core.Item{Key: key, Value: cloneBytes(e.value), ContentType: e.contentType, Version: e.version}, nil
}

// Put replaces the value for the key carried in item. If item.ContentType is empty,
// core.DefaultContentType is used. If item.Version is non-zero and does not match the
// current version, core.ErrVersionMismatch is returned. Version zero means unconditional
// write. On success the version is incremented and the updated Item is returned.
func (s *Store) Put(_ context.Context, item core.Item) (core.Item, error) {
	if item.ContentType == "" {
		item.ContentType = core.DefaultContentType
	}

	e := s.getOrCreate(item.Key)

	e.mu.Lock()
	defer e.mu.Unlock()

	if item.Version != 0 && item.Version != e.version {
		return core.Item{}, core.ErrVersionMismatch
	}

	e.value = cloneBytes(item.Value)
	e.contentType = item.ContentType
	e.version++

	return core.Item{Key: item.Key, Value: cloneBytes(e.value), ContentType: e.contentType, Version: e.version}, nil
}

// getOrCreate returns the existing entry for key, or inserts and returns a new
// zero-value entry. A short write-lock is only taken when the key is absent.
func (s *Store) getOrCreate(key string) *entry {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()

	if ok {
		return e
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check: another goroutine may have inserted while we upgraded.
	if e, ok = s.data[key]; ok {
		return e
	}

	e = &entry{}
	s.data[key] = e

	return e
}

// cloneBytes returns a fresh copy of b, or nil if b is nil.
// Use this whenever a []byte from an untrusted caller is stored or a stored
// []byte is handed out, so that the store's internal state cannot be mutated
// through the returned slice.
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}

	c := make([]byte, len(b))
	copy(c, b)

	return c
}
