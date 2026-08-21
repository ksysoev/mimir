// Package kv provides an in-memory key-value store with per-key atomicity and versioning.
package kv

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
)

// ErrNotFound is returned when a key does not exist in the store.
var ErrNotFound = errors.New("key not found")

// ErrVersionMismatch is returned when an ifVersion guard does not match the current version.
var ErrVersionMismatch = errors.New("version mismatch")

// Item represents a stored key-value pair with its current version.
type Item struct {
	Key     string
	Value   json.RawMessage
	Version int64
}

// entry is the internal per-key structure. It carries its own mutex so that
// concurrent operations on different keys do not block each other.
type entry struct {
	value   json.RawMessage
	version int64
	mu      sync.Mutex
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

// Get returns the Item for key. Returns ErrNotFound if the key does not exist.
func (s *Store) Get(key string) (Item, error) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return Item{}, ErrNotFound
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return Item{Key: key, Value: e.value, Version: e.version}, nil
}

// Put replaces the value for key. If ifVersion is non-nil and does not match
// the current version, ErrVersionMismatch is returned.
// On success the version is incremented and the updated Item is returned.
func (s *Store) Put(key string, value json.RawMessage, ifVersion *int64) (Item, error) {
	e := s.getOrCreate(key)

	e.mu.Lock()
	defer e.mu.Unlock()

	if ifVersion != nil && *ifVersion != e.version {
		return Item{}, ErrVersionMismatch
	}

	e.value = value
	e.version = nextVersion()

	return Item{Key: key, Value: e.value, Version: e.version}, nil
}

// Patch applies delta to the existing value for key.
//   - If the key does not exist, delta is stored as the initial value.
//   - If both existing value and delta are JSON objects, their top-level fields
//     are shallow-merged (delta keys overwrite existing keys).
//   - Otherwise the entire value is replaced with delta (same as Put).
//
// ifVersion semantics are identical to Put.
func (s *Store) Patch(key string, delta json.RawMessage, ifVersion *int64) (Item, error) {
	e := s.getOrCreate(key)

	e.mu.Lock()
	defer e.mu.Unlock()

	if ifVersion != nil && *ifVersion != e.version {
		return Item{}, ErrVersionMismatch
	}

	var newValue json.RawMessage

	switch {
	case e.version == 0:
		// Key is brand-new (version 0 means never written).
		newValue = delta
	case isJSONObject(e.value) && isJSONObject(delta):
		merged, err := shallowMerge(e.value, delta)
		if err != nil {
			return Item{}, err
		}

		newValue = merged
	default:
		newValue = delta
	}

	e.value = newValue
	e.version = nextVersion()

	return Item{Key: key, Value: e.value, Version: e.version}, nil
}

// getOrCreate returns the existing entry for key, or inserts and returns a new
// zero-value entry.  A short write-lock is only taken when the key is absent.
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

// versionSeq is a global monotonic counter used for version numbers.
var versionSeq atomic.Int64

// nextVersion returns the next unique version number.
func nextVersion() int64 {
	return versionSeq.Add(1)
}

// isJSONObject reports whether v is a JSON object (starts with '{').
func isJSONObject(v json.RawMessage) bool {
	for _, b := range v {
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}

		return b == '{'
	}

	return false
}

// shallowMerge overlays the top-level fields of delta onto base and returns
// the resulting JSON object.
func shallowMerge(base, delta json.RawMessage) (json.RawMessage, error) {
	var baseMap map[string]json.RawMessage

	if err := json.Unmarshal(base, &baseMap); err != nil {
		return nil, err
	}

	var deltaMap map[string]json.RawMessage

	if err := json.Unmarshal(delta, &deltaMap); err != nil {
		return nil, err
	}

	for k, v := range deltaMap {
		baseMap[k] = v
	}

	result, err := json.Marshal(baseMap)
	if err != nil {
		return nil, err
	}

	return result, nil
}
