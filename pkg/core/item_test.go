package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- IsJSON ----------------------------------------------------------------

func TestItem_IsJSON_True(t *testing.T) {
	tests := []string{
		"application/json",
		"Application/JSON",
		"application/json; charset=utf-8",
	}

	for _, ct := range tests {
		assert.True(t, (Item{ContentType: ct}).IsJSON(), "expected true for %q", ct)
	}
}

func TestItem_IsJSON_False(t *testing.T) {
	tests := []string{
		"",
		"text/plain",
		"application/octet-stream",
		"not-a-mime-type;;;",
	}

	for _, ct := range tests {
		assert.False(t, (Item{ContentType: ct}).IsJSON(), "expected false for %q", ct)
	}
}

// ---- ValidateJSON ----------------------------------------------------------

func TestItem_ValidateJSON_NonJSONContentType(t *testing.T) {
	item := Item{ContentType: "text/plain", Value: []byte("not json at all")}
	assert.NoError(t, item.ValidateJSON(), "non-JSON content type should skip validation")
}

func TestItem_ValidateJSON_EmptyContentType(t *testing.T) {
	item := Item{ContentType: "", Value: []byte("{bad}")}
	assert.NoError(t, item.ValidateJSON(), "empty content type should skip validation")
}

func TestItem_ValidateJSON_ValidJSON(t *testing.T) {
	cases := [][]byte{
		[]byte(`{}`),
		[]byte(`{"a":1}`),
		[]byte(`[1,2,3]`),
		[]byte(`"hello"`),
		[]byte(`42`),
		[]byte(`true`),
		[]byte(`null`),
	}

	for _, v := range cases {
		item := Item{ContentType: "application/json", Value: v}
		assert.NoError(t, item.ValidateJSON(), "valid JSON %s should pass", v)
	}
}

func TestItem_ValidateJSON_InvalidJSON(t *testing.T) {
	cases := [][]byte{
		[]byte(`{bad}`),
		[]byte(`{`),
		[]byte(``),
		[]byte(`undefined`),
	}

	for _, v := range cases {
		item := Item{ContentType: "application/json", Value: v}
		assert.ErrorIs(t, item.ValidateJSON(), ErrInvalidPayload, "invalid JSON %q should return ErrInvalidPayload", v)
	}
}

func TestItem_ValidateJSON_WithCharsetParam(t *testing.T) {
	item := Item{ContentType: "application/json; charset=utf-8", Value: []byte(`{bad}`)}
	assert.ErrorIs(t, item.ValidateJSON(), ErrInvalidPayload)
}

// ---- ApplyPatch ------------------------------------------------------------

func TestItem_ApplyPatch_NonJSONDelta(t *testing.T) {
	existing := Item{Key: "k", Version: 1, Value: []byte(`{"a":1}`), ContentType: "application/json"}
	delta := Item{ContentType: "text/plain", Value: []byte(`hello`)}

	_, err := existing.ApplyPatch(delta)
	assert.ErrorIs(t, err, ErrUnsupportedContentType)
}

func TestItem_ApplyPatch_InvalidJSONDelta(t *testing.T) {
	existing := Item{Key: "k", Version: 1, Value: []byte(`{"a":1}`), ContentType: "application/json"}
	delta := Item{ContentType: "application/json", Value: []byte(`{bad json`)}

	_, err := existing.ApplyPatch(delta)
	assert.ErrorIs(t, err, ErrInvalidPayload)
}

func TestItem_ApplyPatch_NewKey(t *testing.T) {
	// Version == 0 means the key does not yet exist; delta is stored as-is.
	existing := Item{Key: "k"} // zero value
	delta := Item{ContentType: "application/json", Value: []byte(`{"x":1}`)}

	result, err := existing.ApplyPatch(delta)
	require.NoError(t, err)
	assert.Equal(t, "k", result.Key)
	assert.Equal(t, "application/json", result.ContentType)
	assert.Equal(t, uint64(0), result.Version)
	assert.Equal(t, delta.Value, result.Value)
}

func TestItem_ApplyPatch_MergeObjects(t *testing.T) {
	existing := Item{Key: "k", Version: 3, Value: []byte(`{"a":1,"b":2}`), ContentType: "application/json"}
	delta := Item{ContentType: "application/json", Value: []byte(`{"b":99,"c":3}`)}

	result, err := existing.ApplyPatch(delta)
	require.NoError(t, err)

	var m map[string]int
	require.NoError(t, json.Unmarshal(result.Value, &m))
	assert.Equal(t, 1, m["a"])
	assert.Equal(t, 99, m["b"])
	assert.Equal(t, 3, m["c"])
	assert.Equal(t, "application/json", result.ContentType)
	assert.Equal(t, uint64(3), result.Version, "version must not be advanced by ApplyPatch")
}

func TestItem_ApplyPatch_ReplaceWhenExistingNotObject(t *testing.T) {
	existing := Item{Key: "k", Version: 2, Value: []byte(`42`), ContentType: "application/json"}
	delta := Item{ContentType: "application/json", Value: []byte(`"replaced"`)}

	result, err := existing.ApplyPatch(delta)
	require.NoError(t, err)
	assert.Equal(t, delta.Value, result.Value)
}

func TestItem_ApplyPatch_ReplaceWhenDeltaNotObject(t *testing.T) {
	existing := Item{Key: "k", Version: 1, Value: []byte(`{"a":1}`), ContentType: "application/json"}
	delta := Item{ContentType: "application/json", Value: []byte(`"scalar"`)}

	result, err := existing.ApplyPatch(delta)
	require.NoError(t, err)
	assert.Equal(t, delta.Value, result.Value)
}

func TestItem_ApplyPatch_ReplaceNonJSONExisting(t *testing.T) {
	// existing value is not JSON (e.g. binary blob stored before schema enforcement)
	existing := Item{Key: "k", Version: 1, Value: []byte(`not json`), ContentType: "application/octet-stream"}
	delta := Item{ContentType: "application/json", Value: []byte(`{"replaced":true}`)}

	result, err := existing.ApplyPatch(delta)
	require.NoError(t, err)
	assert.Equal(t, delta.Value, result.Value)
	assert.Equal(t, "application/json", result.ContentType)
}

// ---- internal helpers ------------------------------------------------------

func TestIsJSONObject(t *testing.T) {
	assert.True(t, isJSONObject([]byte(`{}`)))
	assert.True(t, isJSONObject([]byte(`  { "a": 1 }`)))
	assert.False(t, isJSONObject([]byte(`[]`)))
	assert.False(t, isJSONObject([]byte(`"str"`)))
	assert.False(t, isJSONObject([]byte(`42`)))
	assert.False(t, isJSONObject([]byte(``)))
}

func TestShallowMerge(t *testing.T) {
	base := []byte(`{"a":1,"b":2}`)
	delta := []byte(`{"b":99,"c":3}`)

	result, err := shallowMerge(base, delta)
	require.NoError(t, err)

	var m map[string]int
	require.NoError(t, json.Unmarshal(result, &m))
	assert.Equal(t, 1, m["a"])
	assert.Equal(t, 99, m["b"])
	assert.Equal(t, 3, m["c"])
}
