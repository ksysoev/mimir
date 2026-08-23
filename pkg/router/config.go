// Package router implements the mimir cluster router, which distributes
// key-value requests across storage nodes using Rendezvous (HRW) hashing.
package router

import "time"

const (
	defaultHTTPTimeout       = 10 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	// defaultProxyTimeout is the maximum time routeKey will wait for the
	// upstream node to begin sending a response. It is enforced via a
	// per-request context deadline so that a frozen node cannot hold a
	// goroutine indefinitely.
	defaultProxyTimeout = 10 * time.Second
	// maxScanLineBytes is the upper bound on a single NDJSON line returned
	// by a node's GET /kv endpoint. Keys are URL path segments so they are
	// bounded by HTTP URL limits in practice, but we set an explicit 1 MB
	// ceiling to prevent bufio.Scanner from silently dropping long lines
	// with ErrTooLong instead of surfacing a clear error.
	maxScanLineBytes = 1 * 1024 * 1024
)

// NodeConfig identifies a single storage node reachable by the router.
type NodeConfig struct {
	// ID is the stable human-readable node identifier echoed in list-key responses.
	ID string `mapstructure:"id"`
	// URL is the base URL of the node (e.g. "http://node1:7001").
	URL string `mapstructure:"url"`
}

// Config holds all configuration for the router process.
// Field order is optimised for minimal struct padding (govet fieldalignment).
type Config struct {
	Listen      string       `mapstructure:"listen"`
	Key         string       `mapstructure:"key"`
	InternalKey string       `mapstructure:"internal_key"`
	Version     string       `mapstructure:"version"`
	AppName     string       `mapstructure:"app_name"`
	Nodes       []NodeConfig `mapstructure:"nodes"`
	MaxBodySize int64        `mapstructure:"max_body_size"`
}
