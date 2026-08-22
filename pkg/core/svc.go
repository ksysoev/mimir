// Package core provides core service logic and interfaces.
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"strings"
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
// item.Version == 0 performs an unconditional write.
// item.Version > 0 is a CAS guard: ErrVersionMismatch is returned when the
// stored version differs.
func (s *Service) PutKey(ctx context.Context, item Item) (Item, error) {
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
	baseType, _, err := mime.ParseMediaType(item.ContentType)
	if err != nil || !strings.EqualFold(baseType, "application/json") {
		return Item{}, ErrUnsupportedContentType
	}

	if !json.Valid(item.Value) {
		return Item{}, ErrInvalidPayload
	}

	existing, err := s.store.Get(ctx, item.Key)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Item{}, fmt.Errorf("patch %q: get: %w", item.Key, err)
	}

	if item.Version != 0 && existing.Version != item.Version {
		return Item{}, ErrVersionMismatch
	}

	newValue, err := applyPatch(existing, item.Value)
	if err != nil {
		return Item{}, fmt.Errorf("patch %q: merge: %w", item.Key, err)
	}

	result, err := s.store.Put(ctx, Item{Key: item.Key, Value: newValue, ContentType: "application/json", Version: existing.Version})
	if err != nil {
		if errors.Is(err, ErrVersionMismatch) {
			return Item{}, err
		}

		return Item{}, fmt.Errorf("patch %q: put: %w", item.Key, err)
	}

	return result, nil
}
