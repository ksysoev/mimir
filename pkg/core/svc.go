// Package core provides core service logic and interfaces.
package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/ksysoev/mimir/pkg/repo/kv"
)

// kvStore defines the interface for key-value storage operations.
type kvStore interface {
	Get(key string) (kv.Item, error)
	Put(key string, value []byte, contentType string, ifVersion *int64) (kv.Item, error)
	Patch(key string, delta []byte, ifVersion *int64) (kv.Item, error)
}

// Service encapsulates core business logic.
type Service struct {
	store kvStore
}

// New creates a new Service backed by the provided kvStore.
func New(store kvStore) *Service {
	return &Service{store: store}
}

// CheckHealth always returns nil for the in-memory store.
func (s *Service) CheckHealth(_ context.Context) error {
	return nil
}

// GetKey retrieves the item stored at key.
// Returns kv.ErrNotFound if the key does not exist.
func (s *Service) GetKey(_ context.Context, key string) (kv.Item, error) {
	item, err := s.store.Get(key)
	if err != nil {
		return kv.Item{}, fmt.Errorf("get %q: %w", key, err)
	}

	return item, nil
}

// PutKey replaces the value at key.
// Returns kv.ErrVersionMismatch if ifVersion is supplied and mismatches.
func (s *Service) PutKey(_ context.Context, key string, value []byte, contentType string, ifVersion *int64) (kv.Item, error) {
	item, err := s.store.Put(key, value, contentType, ifVersion)
	if err != nil {
		if errors.Is(err, kv.ErrVersionMismatch) {
			return kv.Item{}, err
		}

		return kv.Item{}, fmt.Errorf("put %q: %w", key, err)
	}

	return item, nil
}

// PatchKey applies delta to the value at key using shallow-merge semantics.
// delta must be valid JSON; callers are responsible for validating this.
// Returns kv.ErrVersionMismatch if ifVersion is supplied and mismatches.
func (s *Service) PatchKey(_ context.Context, key string, delta []byte, ifVersion *int64) (kv.Item, error) {
	item, err := s.store.Patch(key, delta, ifVersion)
	if err != nil {
		if errors.Is(err, kv.ErrVersionMismatch) {
			return kv.Item{}, err
		}

		return kv.Item{}, fmt.Errorf("patch %q: %w", key, err)
	}

	return item, nil
}
