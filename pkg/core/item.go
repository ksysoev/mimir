package core

import (
	"encoding/json"
	"mime"
	"strings"
)

// Item represents a stored key-value pair with its current version and content type.
type Item struct {
	Key         string
	ContentType string
	Value       []byte
	Version     uint64
}

// IsJSON reports whether the item's ContentType is application/json.
// The check is MIME-parsed and case-insensitive, so values like
// "application/json; charset=utf-8" are also accepted.
func (item Item) IsJSON() bool {
	baseType, _, err := mime.ParseMediaType(item.ContentType)
	if err != nil {
		return false
	}

	return strings.EqualFold(baseType, "application/json")
}

// ValidateJSON returns ErrInvalidPayload when the item's ContentType is
// application/json and Value is not valid JSON.
// For all other content types it is a no-op and returns nil.
func (item Item) ValidateJSON() error {
	if !item.IsJSON() {
		return nil
	}

	if !json.Valid(item.Value) {
		return ErrInvalidPayload
	}

	return nil
}

// ApplyPatch merges delta into item and returns a new Item ready to be persisted.
//
// Validation (returns an error without touching the store):
//   - delta.ContentType must be JSON (application/json, MIME-parsed) → ErrUnsupportedContentType
//   - delta.Value must be valid JSON                                 → ErrInvalidPayload
//
// Merge semantics:
//   - item.Version == 0 (key does not yet exist): delta is stored as-is.
//   - Both item.Value and delta.Value are JSON objects: shallow-merge (delta
//     keys overwrite item keys).
//   - Otherwise: delta replaces item.Value entirely.
//
// The returned Item carries item.Key, item.Version (unchanged — the store
// advances the version on write), and ContentType "application/json".
func (item Item) ApplyPatch(delta Item) (Item, error) {
	if !delta.IsJSON() {
		return Item{}, ErrUnsupportedContentType
	}

	if !json.Valid(delta.Value) {
		return Item{}, ErrInvalidPayload
	}

	newValue, err := applyMerge(item, delta.Value)
	if err != nil {
		return Item{}, err
	}

	return Item{
		Key:         item.Key,
		ContentType: "application/json",
		Value:       newValue,
		Version:     item.Version,
	}, nil
}

// applyMerge computes the new raw value when patching existing with delta bytes.
//
//   - If existing.Version == 0 (brand-new key), delta is returned as-is.
//   - If both existing.Value and delta are JSON objects, shallow-merge is applied.
//   - Otherwise delta replaces existing.Value entirely.
func applyMerge(existing Item, delta []byte) ([]byte, error) {
	switch {
	case existing.Version == 0:
		return delta, nil
	case isJSONObject(existing.Value) && isJSONObject(delta):
		return shallowMerge(existing.Value, delta)
	default:
		return delta, nil
	}
}

// isJSONObject reports whether v is a JSON object (first non-whitespace byte is '{').
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
// the resulting JSON object. delta keys overwrite base keys.
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

	return json.Marshal(baseMap)
}
