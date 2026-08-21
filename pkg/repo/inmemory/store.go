// Package inmemory provides an in-memory key-value store with per-key atomicity and versioning.
package inmemory

import (
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/ksysoev/mimir/pkg/core"
)

// entry is the internal per-key structure. It carries its own mutex so that
// concurrent operations on different keys do not block each other.
type entry struct {
	contentType string
	value       []byte
	version     int64
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
func (s *Store) Get(key string) (core.Item, error) {
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

// Put replaces the value for key. contentType is stored alongside the value and
// returned on subsequent Get calls. If contentType is empty, core.DefaultContentType
// is used. If ifVersion is non-nil and does not match the current version,
// core.ErrVersionMismatch is returned. On success the version is incremented and the
// updated Item is returned.
func (s *Store) Put(key string, value []byte, contentType string, ifVersion *int64) (core.Item, error) {
	if contentType == "" {
		contentType = core.DefaultContentType
	}

	e := s.getOrCreate(key)

	e.mu.Lock()
	defer e.mu.Unlock()

	if ifVersion != nil && *ifVersion != e.version {
		return core.Item{}, core.ErrVersionMismatch
	}

	e.value = cloneBytes(value)
	e.contentType = contentType
	e.version = nextVersion()

	return core.Item{Key: key, Value: cloneBytes(e.value), ContentType: e.contentType, Version: e.version}, nil
}

// Patch applies a JSON delta to the existing value for key. The delta must be
// valid JSON; callers are responsible for enforcing this before calling Patch.
//
//   - If the key does not exist, delta is stored as the initial value.
//   - If both existing value and delta are JSON objects, their top-level fields
//     are shallow-merged (delta keys overwrite existing keys).
//   - Otherwise the entire value is replaced with delta (same as Put).
//
// The stored content type is always set to "application/json" after a patch.
// ifVersion semantics are identical to Put.
func (s *Store) Patch(key string, delta []byte, ifVersion *int64) (core.Item, error) {
	e := s.getOrCreate(key)

	e.mu.Lock()
	defer e.mu.Unlock()

	if ifVersion != nil && *ifVersion != e.version {
		return core.Item{}, core.ErrVersionMismatch
	}

	var newValue []byte

	switch {
	case e.version == 0:
		// Key is brand-new (version 0 means never written).
		newValue = delta
	case isJSONObject(e.value) && isJSONObject(delta):
		merged, err := shallowMerge(e.value, delta)
		if err != nil {
			return core.Item{}, err
		}

		newValue = merged
	default:
		newValue = delta
	}

	e.value = newValue
	e.contentType = "application/json"
	e.version = nextVersion()

	return core.Item{Key: key, Value: cloneBytes(e.value), ContentType: e.contentType, Version: e.version}, nil
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

// isJSONObject reports whether v is a JSON object (starts with '{').
func isJSONObject(v []byte) bool {
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
func shallowMerge(base, delta []byte) ([]byte, error) {
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
