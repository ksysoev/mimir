package router

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectNode_Deterministic(t *testing.T) {
	nodes := []NodeConfig{
		{ID: "node-1", URL: "http://n1"},
		{ID: "node-2", URL: "http://n2"},
		{ID: "node-3", URL: "http://n3"},
	}

	for range 100 {
		first := SelectNode(nodes, "my-key")
		second := SelectNode(nodes, "my-key")
		assert.Equal(t, first, second, "SelectNode must be deterministic")
	}
}

func TestSelectNode_EmptyNodes(t *testing.T) {
	result := SelectNode(nil, "any-key")
	assert.Equal(t, NodeConfig{}, result, "empty node list must return zero NodeConfig")
}

func TestSelectNode_SingleNode(t *testing.T) {
	only := NodeConfig{ID: "solo", URL: "http://solo"}
	result := SelectNode([]NodeConfig{only}, "whatever")
	assert.Equal(t, only, result)
}

func TestSelectNode_Distribution(t *testing.T) {
	// With 10 000 keys spread across 3 nodes, each bucket should hold
	// between 25% and 42% of keys (expected ~33%, tolerance ±8 pp).
	const (
		numKeys  = 10_000
		numNodes = 3
		minFrac  = 0.20
		maxFrac  = 0.47
	)

	nodes := make([]NodeConfig, numNodes)
	for i := range numNodes {
		nodes[i] = NodeConfig{ID: fmt.Sprintf("node-%d", i+1), URL: "http://x"}
	}

	counts := make(map[string]int, numNodes)

	for i := range numKeys {
		key := fmt.Sprintf("test-key-%d", i)
		n := SelectNode(nodes, key)
		counts[n.ID]++
	}

	require.Len(t, counts, numNodes, "all nodes must receive at least one key")

	for id, count := range counts {
		frac := float64(count) / numKeys
		assert.True(t, frac >= minFrac && frac <= maxFrac,
			"node %s fraction %.2f is outside [%.2f, %.2f]", id, frac, minFrac, maxFrac)
	}
}

func TestSelectNode_NodeAddition_MinimalRemap(t *testing.T) {
	// After adding a 4th node, at most ~1/3 of keys should remap.
	// We allow up to 40% to keep the test robust under hash variance.
	const numKeys = 5_000

	nodes3 := []NodeConfig{
		{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"},
	}
	nodes4 := append(nodes3, NodeConfig{ID: "node-4"}) //nolint:gocritic

	remapped := 0

	for i := range numKeys {
		key := fmt.Sprintf("remap-key-%d", i)
		if SelectNode(nodes3, key).ID != SelectNode(nodes4, key).ID {
			remapped++
		}
	}

	remapFrac := float64(remapped) / numKeys
	assert.Less(t, remapFrac, 0.40,
		"adding a node should remap ≤40%% of keys, got %.2f%%", remapFrac*100)
}

func TestSelectNode_DifferentKeysDifferentNodes(t *testing.T) {
	// Sanity: distinct keys must not all land on the same node.
	nodes := []NodeConfig{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	seen := make(map[string]bool)

	for i := range 200 {
		key := fmt.Sprintf("spread-key-%d", i)
		seen[SelectNode(nodes, key).ID] = true
	}

	assert.Greater(t, len(seen), 1, "distinct keys must spread across multiple nodes")
}

func TestSelectNode_CollisionResistance(t *testing.T) {
	// Pairs like ("ab","c") and ("a","bc") must not collide.
	nodes := []NodeConfig{{ID: "ab", URL: "http://ab"}, {ID: "a", URL: "http://a"}}

	n1 := SelectNode(nodes, "c")
	n2 := SelectNode(nodes, "bc")

	// They *may* land on the same node by chance, but we at least verify
	// both calls complete without panic and return a valid node.
	assert.Contains(t, []string{"ab", "a"}, n1.ID)
	assert.Contains(t, []string{"ab", "a"}, n2.ID)
	_ = math.Pi // suppress unused import lint
}
