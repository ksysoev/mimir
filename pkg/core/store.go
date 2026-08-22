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

// ErrStoreFull is returned when the store has reached its maximum number of keys.
var ErrStoreFull = errors.New("store is full")
