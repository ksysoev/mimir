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

// PutKey replaces the value at key.
// Returns ErrVersionMismatch if req.IfVersion is supplied and mismatches the stored version.
func (s *Service) PutKey(ctx context.Context, req PutRequest) (Item, error) {
	var version uint64
	if req.IfVersion != nil {
		version = *req.IfVersion
	}

	item, err := s.store.Put(ctx, Item{Key: req.Key, Value: req.Value, ContentType: req.ContentType, Version: version})
	if err != nil {
		if errors.Is(err, ErrVersionMismatch) {
			return Item{}, err
		}

		return Item{}, fmt.Errorf("put %q: %w", req.Key, err)
	}

	return item, nil
}

// PatchKey applies req.Delta to the value at req.Key using shallow-merge semantics.
// Returns ErrUnsupportedContentType if the Content-Type is not application/json.
// Returns ErrInvalidPayload if req.Delta is not valid JSON.
// Returns ErrVersionMismatch if req.IfVersion is supplied and mismatches the
// current version, or if a concurrent write races the internal Get→Put.
func (s *Service) PatchKey(ctx context.Context, req PatchRequest) (Item, error) {
	baseType, _, err := mime.ParseMediaType(req.ContentType)
	if err != nil || !strings.EqualFold(baseType, "application/json") {
		return Item{}, ErrUnsupportedContentType
	}

	if !json.Valid(req.Delta) {
		return Item{}, ErrInvalidPayload
	}

	existing, err := s.store.Get(ctx, req.Key)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Item{}, fmt.Errorf("patch %q: get: %w", req.Key, err)
	}

	if req.IfVersion != nil && existing.Version != *req.IfVersion {
		return Item{}, ErrVersionMismatch
	}

	newValue, err := applyPatch(existing, req.Delta)
	if err != nil {
		return Item{}, fmt.Errorf("patch %q: merge: %w", req.Key, err)
	}

	item, err := s.store.Put(ctx, Item{Key: req.Key, Value: newValue, ContentType: "application/json", Version: existing.Version})
	if err != nil {
		if errors.Is(err, ErrVersionMismatch) {
			return Item{}, err
		}

		return Item{}, fmt.Errorf("patch %q: put: %w", req.Key, err)
	}

	return item, nil
}
