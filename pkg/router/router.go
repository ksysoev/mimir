package router

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ksysoev/mimir/pkg/api/middleware"
)

// New validates cfg and returns a ready-to-run Router.
// Returns an error if no nodes are configured or the listen address is empty.
func New(cfg *Config) (*Router, error) {
	if cfg.Listen == "" {
		return nil, fmt.Errorf("router listen address must be specified")
	}

	if len(cfg.Nodes) == 0 {
		return nil, fmt.Errorf("router requires at least one node")
	}

	return &Router{
		nodes:        cfg.Nodes,
		internalKey:  cfg.InternalKey,
		proxyTimeout: defaultProxyTimeout,
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}, nil
}

// Run starts the router HTTP server and blocks until ctx is cancelled.
// The middleware stack applied here mirrors the node API layer:
//
//   - NewReqID    — attaches a unique request ID to every request context.
//   - NewAPIKey   — validates the client-facing X-API-Key on KV routes.
//   - NewSanitize — enforces max body size on mutating routes.
//
// GET /livez carries only NewReqID so liveness probes work without credentials.
// GET /kv carries NewReqID + NewAPIKey but not NewSanitize (no body).
func (r *Router) Run(ctx context.Context, cfg *Config) error {
	withReqID := middleware.NewReqID()
	withAPIKey := middleware.NewAPIKey(cfg.Key)
	withSanitize := middleware.NewSanitize(cfg.MaxBodySize)

	mux := http.NewServeMux()
	mux.Handle("GET /livez", middleware.Use(r.healthCheck, withReqID))
	mux.Handle("GET /kv", middleware.Use(r.listKeys, withReqID, withAPIKey))
	mux.Handle("GET /kv/{key}", middleware.Use(r.routeKey, withReqID, withAPIKey, withSanitize))
	mux.Handle("PUT /kv/{key}", middleware.Use(r.routeKey, withReqID, withAPIKey, withSanitize))
	mux.Handle("PATCH /kv/{key}", middleware.Use(r.routeKey, withReqID, withAPIKey, withSanitize))

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		WriteTimeout:      defaultWriteTimeout,
	}

	go func() {
		<-ctx.Done()

		// Use WithoutCancel so the shutdown deadline is not immediately
		// cancelled along with ctx — we need a short window to drain connections.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
		}
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}
