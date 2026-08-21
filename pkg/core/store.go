package core

import "errors"

// DefaultContentType is used when no content type is provided on Put.
const DefaultContentType = "application/octet-stream"

// ErrNotFound is returned when a key does not exist in the store.
var ErrNotFound = errors.New("key not found")

// ErrVersionMismatch is returned when an ifVersion guard does not match the current version.
var ErrVersionMismatch = errors.New("version mismatch")

// ErrUnsupportedContentType is returned when the operation does not support the provided content type.
var ErrUnsupportedContentType = errors.New("unsupported content type")

// ErrInvalidPayload is returned when the provided payload is malformed or invalid for the operation.
var ErrInvalidPayload = errors.New("invalid payload")

// Item represents a stored key-value pair with its current version and content type.
type Item struct {
	Key         string
	ContentType string
	Value       []byte
	Version     uint64
}

// PutRequest carries the data needed to create or replace a key.
// IfVersion, when non-nil, enables optimistic locking: the write is rejected
// with ErrVersionMismatch if the stored version differs.
type PutRequest struct {
	Item
	IfVersion *uint64
}

// PatchRequest carries the data needed to partially update a key.
// ContentType must be "application/json"; other values are rejected with ErrUnsupportedContentType.
// Delta must be valid JSON; invalid JSON is rejected with ErrInvalidPayload.
// IfVersion, when non-nil, enables optimistic locking.
type PatchRequest struct {
	Key         string
	ContentType string
	Delta       []byte
	IfVersion   *uint64
}
