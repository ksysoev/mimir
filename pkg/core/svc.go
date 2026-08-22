// Package core provides core service logic and interfaces.
package core

import (
	"context"
	"errors"
	"fmt"
)

// kvStore defines the interface for key-value storage operations.
type kvStore interface {
	Get(ctx context.Context, key string) (Item, error)
	Put(ctx context.Context, item Item) (Item, error)
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
func (s *Service) GetKey(ctx context.Context, key string) (Item, error) {
	item, err := s.store.Get(ctx, key)
	if err != nil {
		return Item{}, fmt.Errorf("get %q: %w", key, err)
	}

	return item, nil
}

// PutKey replaces the value at item.Key.
// When item.ContentType is application/json, item.Value must be valid JSON;
// invalid JSON returns ErrInvalidPayload.
// item.Version == 0 performs an unconditional write.
// item.Version > 0 is a CAS guard: ErrVersionMismatch is returned when the
// stored version differs.
func (s *Service) PutKey(ctx context.Context, item Item) (Item, error) {
	if err := item.ValidateJSON(); err != nil {
		return Item{}, err
	}

	result, err := s.store.Put(ctx, item)
	if err != nil {
		if errors.Is(err, ErrVersionMismatch) {
			return Item{}, err
		}

		return Item{}, fmt.Errorf("put %q: %w", item.Key, err)
	}

	return result, nil
}

// PatchKey applies item.Value (the patch delta) to the existing value at item.Key
// using shallow-merge semantics. item.ContentType must be "application/json";
// other values return ErrUnsupportedContentType. item.Value must be valid JSON;
// invalid JSON returns ErrInvalidPayload.
// item.Version == 0 skips the caller version guard.
// item.Version > 0 acts as a CAS guard against the current stored version;
// ErrVersionMismatch is returned when the versions differ or a concurrent
// write races the internal Get→Put.
func (s *Service) PatchKey(ctx context.Context, item Item) (Item, error) {
	// Fail fast before any I/O — ApplyPatch re-validates defensively for
	// callers that bypass the service layer directly.
	if !item.IsJSON() {
		return Item{}, ErrUnsupportedContentType
	}

	if err := item.ValidateJSON(); err != nil {
		return Item{}, err
	}

	existing, err := s.store.Get(ctx, item.Key)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return Item{}, fmt.Errorf("patch %q: get: %w", item.Key, err)
		}

		// Key does not exist yet; seed Key so ApplyPatch can propagate it.
		existing = Item{Key: item.Key}
	}

	if item.Version != 0 && existing.Version != item.Version {
		return Item{}, ErrVersionMismatch
	}

	merged, err := existing.ApplyPatch(item)
	if err != nil {
		return Item{}, fmt.Errorf("patch %q: %w", item.Key, err)
	}

	result, err := s.store.Put(ctx, merged)
	if err != nil {
		if errors.Is(err, ErrVersionMismatch) {
			return Item{}, err
		}

		return Item{}, fmt.Errorf("patch %q: put: %w", item.Key, err)
	}

	return result, nil
}
