package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPatch_NewKey(t *testing.T) {
	delta := []byte(`{"x":1}`)
	result, err := applyPatch(Item{}, delta)
	require.NoError(t, err)
	assert.Equal(t, delta, result)
}

func TestApplyPatch_MergeObjects(t *testing.T) {
	existing := Item{Value: []byte(`{"a":1,"b":2}`), Version: 1}
	delta := []byte(`{"b":99,"c":3}`)

	result, err := applyPatch(existing, delta)
	require.NoError(t, err)

	var m map[string]int
	require.NoError(t, json.Unmarshal(result, &m))
	assert.Equal(t, 1, m["a"])
	assert.Equal(t, 99, m["b"])
	assert.Equal(t, 3, m["c"])
}

func TestApplyPatch_ReplaceWhenExistingNotObject(t *testing.T) {
	existing := Item{Value: []byte(`42`), Version: 1}
	delta := []byte(`"replaced"`)

	result, err := applyPatch(existing, delta)
	require.NoError(t, err)
	assert.Equal(t, delta, result)
}

func TestApplyPatch_ReplaceWhenDeltaNotObject(t *testing.T) {
	existing := Item{Value: []byte(`{"a":1}`), Version: 1}
	delta := []byte(`"scalar"`)

	result, err := applyPatch(existing, delta)
	require.NoError(t, err)
	assert.Equal(t, delta, result)
}

func TestApplyPatch_ReplaceNonJSONExisting(t *testing.T) {
	existing := Item{Value: []byte(`not json`), ContentType: "application/octet-stream", Version: 1}
	delta := []byte(`{"replaced":true}`)

	result, err := applyPatch(existing, delta)
	require.NoError(t, err)
	assert.Equal(t, delta, result)
}

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
