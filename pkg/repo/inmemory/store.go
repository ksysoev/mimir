// Package inmemory provides an in-memory key-value store with per-key atomicity and versioning.
package inmemory

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ksysoev/mimir/pkg/core"
)

const (
	// DefaultMaxKeys is the default maximum number of keys the store will hold.
	DefaultMaxKeys = 1000
)

// Config holds configuration for the in-memory store.
type Config struct {
	// MaxKeys is the maximum number of distinct keys the store will keep in memory.
	// A value of 0 uses DefaultMaxKeys.
	MaxKeys int `mapstructure:"max_keys"`
}

// entryVal is an immutable snapshot of a single key's state. A new pointer is
// created on every write; the old one is replaced atomically via sync.Map CAS.
// Because it is never mutated after being stored, it is safe to read without a lock.
// Fields are ordered by size to satisfy fieldalignment: uint64 first, then slice, then string.
type entryVal struct {
	contentType string
	value       []byte
	version     uint64
}

// Store is an in-memory key-value store backed by sync.Map.
// All map-level synchronisation is handled by sync.Map; per-key write
// atomicity is achieved through a CAS loop rather than per-entry mutexes.
type Store struct {
	data    sync.Map
	count   atomic.Int64
	maxKeys int
}

// NewStore creates and returns an empty Store configured via cfg.
// When cfg.MaxKeys is 0, DefaultMaxKeys is used.
func NewStore(cfg Config) *Store {
	maxKeys := cfg.MaxKeys
	if maxKeys <= 0 {
		maxKeys = DefaultMaxKeys
	}

	return &Store{maxKeys: maxKeys}
}

// Get returns the Item for key. Returns core.ErrNotFound if the key does not exist.
// The read is fully lock-free once the key has been promoted to sync.Map's read map.
func (s *Store) Get(_ context.Context, key string) (core.Item, error) {
	v, ok := s.data.Load(key)
	if !ok {
		return core.Item{}, core.ErrNotFound
	}

	ev, ok := v.(*entryVal)
	if !ok {
		return core.Item{}, core.ErrNotFound
	}

	// ev is immutable, no lock required.
	return core.Item{
		Key:         key,
		Value:       cloneBytes(ev.value),
		ContentType: ev.contentType,
		Version:     ev.version,
	}, nil
}

// Put replaces the value for the key carried in item. If item.ContentType is empty,
// core.DefaultContentType is used. If item.Version is non-zero and does not match the
// current version, core.ErrVersionMismatch is returned. Version zero means unconditional
// write. When the key is new and the store has reached its MaxKeys limit,
// core.ErrStoreFull is returned. On success the version is incremented and the updated
// Item is returned.
//
// Atomicity is achieved via a CAS loop: each iteration reads the current snapshot,
// validates the version, builds a new immutable snapshot, and attempts to swap it in.
// Only one goroutine can win each CAS; losers retry with the updated value.
func (s *Store) Put(_ context.Context, item core.Item) (core.Item, error) {
	if item.ContentType == "" {
		item.ContentType = core.DefaultContentType
	}

	newBytes := cloneBytes(item.Value)

	var winner *entryVal

	for {
		current, exists := s.data.Load(item.Key)

		if exists {
			ev, ok := current.(*entryVal)
			if !ok {
				return core.Item{}, core.ErrNotFound
			}

			if item.Version != 0 && item.Version != ev.version {
				return core.Item{}, core.ErrVersionMismatch
			}

			candidate := &entryVal{
				version:     ev.version + 1,
				value:       newBytes,
				contentType: item.ContentType,
			}

			if s.data.CompareAndSwap(item.Key, current, candidate) {
				winner = candidate

				break
			}

			// Another goroutine swapped in a new value; retry.
			continue
		}

		// Key does not exist yet; enforce the capacity limit before inserting.
		if s.count.Load() >= int64(s.maxKeys) {
			return core.Item{}, core.ErrStoreFull
		}

		candidate := &entryVal{
			version:     1,
			value:       newBytes,
			contentType: item.ContentType,
		}

		actual, loaded := s.data.LoadOrStore(item.Key, candidate)
		if !loaded {
			// We inserted the first value.
			s.count.Add(1)

			winner = candidate

			break
		}

		// Another goroutine inserted between our Load and LoadOrStore.
		// Treat it as an existing key and retry the CAS path.
		_ = actual
	}

	return core.Item{
		Key:         item.Key,
		Value:       cloneBytes(winner.value),
		ContentType: winner.contentType,
		Version:     winner.version,
	}, nil
}

// ListKeys returns a point-in-time snapshot of all key names held in the store.
func (s *Store) ListKeys(_ context.Context) []string {
	var keys []string

	s.data.Range(func(k, _ any) bool {
		if key, ok := k.(string); ok {
			keys = append(keys, key)
		}

		return true
	})

	return keys
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
