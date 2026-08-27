// Package inmemory provides an in-memory key-value store with per-key atomicity and versioning.
package inmemory

import (
	"context"
	"fmt"
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
// Fields are ordered to satisfy fieldalignment: string first, then slice, then uint64.
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

	ev := mustEntryVal(v)

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
//
// The MaxKeys limit is enforced as a hard cap using a reserve-first pattern:
// a slot is atomically reserved before LoadOrStore, and released if the key
// turns out to already exist or if the limit is exceeded.
func (s *Store) Put(_ context.Context, item core.Item) (core.Item, error) {
	if item.ContentType == "" {
		item.ContentType = core.DefaultContentType
	}

	newBytes := cloneBytes(item.Value)

	var winner *entryVal

	for {
		current, exists := s.data.Load(item.Key)

		if exists {
			ev := mustEntryVal(current)

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

		// Key does not exist yet. A conditional write (non-zero version) against a
		// missing key is always a mismatch: there is no current version to match.
		if item.Version != 0 {
			return core.Item{}, core.ErrVersionMismatch
		}

		// Reserve a slot atomically before inserting so
		// that MaxKeys is a strict hard cap even under concurrent inserts.
		if s.count.Add(1) > int64(s.maxKeys) {
			// Limit exceeded; roll back the reservation.
			s.count.Add(-1)

			return core.Item{}, core.ErrStoreFull
		}

		candidate := &entryVal{
			version:     1,
			value:       newBytes,
			contentType: item.ContentType,
		}

		_, loaded := s.data.LoadOrStore(item.Key, candidate)
		if !loaded {
			// We won the insert; reservation is consumed.
			winner = candidate

			break
		}

		// Another goroutine inserted this key between our Load and LoadOrStore.
		// Release the reservation and retry via the existing-key CAS path.
		s.count.Add(-1)
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
		keys = append(keys, mustKey(k))

		return true
	})

	return keys
}

// mustEntryVal extracts an *entryVal from a sync.Map value.
// It panics if v is not *entryVal — this would indicate a programming bug
// since the store is the sole writer and always stores *entryVal values.
func mustEntryVal(v any) *entryVal {
	ev, ok := v.(*entryVal)
	if !ok {
		panic(fmt.Sprintf("inmemory: unexpected value type %T stored in sync.Map, want *entryVal", v))
	}

	return ev
}

// mustKey extracts a string key from a sync.Map key value.
// It panics if k is not a string — this would indicate a programming bug
// since the store always uses string keys.
func mustKey(k any) string {
	key, ok := k.(string)
	if !ok {
		panic(fmt.Sprintf("inmemory: unexpected key type %T in sync.Map, want string", k))
	}

	return key
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
