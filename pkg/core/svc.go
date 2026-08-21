// Package core provides core service logic and interfaces.
package core

import (
	"context"
	"errors"
	"fmt"
)

// kvStore defines the interface for key-value storage operations.
type kvStore interface {
	Get(key string) (Item, error)
	Put(key string, value []byte, contentType string, ifVersion *int64) (Item, error)
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
// Returns ErrNotFound if the key does not exist.
func (s *Service) GetKey(_ context.Context, key string) (Item, error) {
	item, err := s.store.Get(key)
	if err != nil {
		return Item{}, fmt.Errorf("get %q: %w", key, err)
	}

	return item, nil
}

// PutKey replaces the value at key.
// Returns ErrVersionMismatch if ifVersion is supplied and mismatches.
func (s *Service) PutKey(_ context.Context, key string, value []byte, contentType string, ifVersion *int64) (Item, error) {
	item, err := s.store.Put(key, value, contentType, ifVersion)
	if err != nil {
		if errors.Is(err, ErrVersionMismatch) {
			return Item{}, err
		}

		return Item{}, fmt.Errorf("put %q: %w", key, err)
	}

	return item, nil
}

// PatchKey applies delta to the value at key using shallow-merge semantics.
// delta must be valid JSON; callers are responsible for validating this.
// Returns ErrVersionMismatch if ifVersion is supplied and mismatches the
// current version, or if a concurrent write races the internal Get→Put.
func (s *Service) PatchKey(_ context.Context, key string, delta []byte, ifVersion *int64) (Item, error) {
	existing, err := s.store.Get(key)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Item{}, fmt.Errorf("patch %q: get: %w", key, err)
	}

	if ifVersion != nil && existing.Version != *ifVersion {
		return Item{}, ErrVersionMismatch
	}

	newValue, err := applyPatch(existing, delta)
	if err != nil {
		return Item{}, fmt.Errorf("patch %q: merge: %w", key, err)
	}

	item, err := s.store.Put(key, newValue, "application/json", &existing.Version)
	if err != nil {
		if errors.Is(err, ErrVersionMismatch) {
			return Item{}, err
		}

		return Item{}, fmt.Errorf("patch %q: put: %w", key, err)
	}

	return item, nil
}
