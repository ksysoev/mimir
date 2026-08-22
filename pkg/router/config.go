// Package router implements the mimir cluster router, which distributes
// key-value requests across storage nodes using Rendezvous (HRW) hashing.
package router

import "time"

const (
	defaultHTTPTimeout      = 10 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
	defaultWriteTimeout     = 15 * time.Second
)

// NodeConfig identifies a single storage node reachable by the router.
type NodeConfig struct {
	// ID is the stable human-readable node identifier echoed in list-key responses.
	ID string `mapstructure:"id"`
	// URL is the base URL of the node (e.g. "http://node1:7001").
	URL string `mapstructure:"url"`
}

// Config holds all configuration for the router process.
type Config struct {
	// Listen is the address the router binds to (e.g. ":7000").
	Listen string `mapstructure:"listen"`
	// Key is the API key required from external clients in the X-API-Key header.
	Key string `mapstructure:"key"`
	// InternalKey is the API key forwarded to storage nodes. It can (and should)
	// differ from the client-facing Key so node ports do not need to be published.
	InternalKey string `mapstructure:"internal_key"`
	// MaxBodySize is the maximum request body the router will accept, in bytes.
	// Defaults to the middleware package default (10 KB) when 0.
	MaxBodySize int64 `mapstructure:"max_body_size"`
	// Nodes is the ordered list of storage nodes the router can route to.
	Nodes []NodeConfig `mapstructure:"nodes"`
}
