package core

import "errors"

// DefaultContentType is used when no content type is provided on Put.
const DefaultContentType = "application/octet-stream"

// ErrNotFound is returned when a key does not exist in the store.
var ErrNotFound = errors.New("key not found")

// ErrVersionMismatch is returned when an ifVersion guard does not match the current version.
var ErrVersionMismatch = errors.New("version mismatch")

// Item represents a stored key-value pair with its current version and content type.
type Item struct {
	Key         string
	ContentType string
	Value       []byte
	Version     uint64
}
