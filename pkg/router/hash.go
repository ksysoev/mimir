package router

import (
	"fmt"
	"hash/fnv"
)

// SelectNode returns the node responsible for key using Rendezvous /
// Highest-Random-Weight (HRW) hashing.
//
// Properties:
//   - Deterministic: identical (nodes, key) inputs always produce the same node.
//   - Minimal disruption: adding or removing a node remaps only ~1/N keys.
//   - No state: stateless pure function, safe for concurrent use.
//
// Returns the zero NodeConfig if nodes is empty; callers must guard against this.
func SelectNode(nodes []NodeConfig, key string) NodeConfig {
	if len(nodes) == 0 {
		return NodeConfig{}
	}

	var (
		best      NodeConfig
		bestScore uint64
	)

	for _, n := range nodes {
		h := fnv.New64a()
		// The \x00 separator prevents hash collisions between node-key pairs
		// that would otherwise concatenate to the same byte string,
		// e.g. (nodeID="ab", key="c") vs (nodeID="a", key="bc").
		fmt.Fprintf(h, "%s\x00%s", n.ID, key)

		if score := h.Sum64(); score > bestScore {
			bestScore = score
			best = n
		}
	}

	return best
}
