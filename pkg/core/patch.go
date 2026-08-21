package core

import "encoding/json"

// applyPatch computes the new value to store when patching key.
//
//   - If the key is brand-new (existing.Version == 0), delta is stored as-is.
//   - If both the existing value and delta are JSON objects, their top-level
//     fields are shallow-merged (delta keys overwrite existing keys).
//   - Otherwise delta replaces the existing value entirely.
func applyPatch(existing Item, delta []byte) ([]byte, error) {
	switch {
	case existing.Version == 0:
		return delta, nil
	case isJSONObject(existing.Value) && isJSONObject(delta):
		return shallowMerge(existing.Value, delta)
	default:
		return delta, nil
	}
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
