// Package api provides the implementation of the API server for the application.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ksysoev/mimir/pkg/core"
)

const (
	defaultTimeout = 5 * time.Second
)

type API struct {
	svc    Service
	config Config
}

type Config struct {
	Listen string `mapstructure:"listen"`
	// APIKey is the static key required in the X-API-Key header on every KV
	// request. Leave empty to disable authentication.
	APIKey string `mapstructure:"api_key"`
	// MaxBodySize is the maximum request body size in bytes.
	// Defaults to middleware.DefaultMaxBodySize (10 KB) when 0.
	MaxBodySize int64 `mapstructure:"max_body_size"`
}

type Service interface {
	CheckHealth(ctx context.Context) error
	GetKey(ctx context.Context, key string) (core.Item, error)
	PutKey(ctx context.Context, item core.Item) (core.Item, error)
	PatchKey(ctx context.Context, item core.Item) (core.Item, error)
}

// New creates a new API instance with the provided configuration and service.
// It validates the configuration and returns an error if the listen address is not specified.
func New(cfg Config, svc Service) (*API, error) {
	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address must be specified")
	}

	api := &API{
		config: cfg,
		svc:    svc,
	}

	return api, nil
}

// Run starts the API server with the provided configuration.
// It listens on the address specified in the configuration and handles graceful shutdown.
// The server will log any errors encountered during shutdown.
// If the server fails to start, it returns an error.
func (a *API) Run(ctx context.Context) error {
	s := &http.Server{
		Addr:              a.config.Listen,
		ReadHeaderTimeout: defaultTimeout,
		WriteTimeout:      defaultTimeout,
		Handler:           a.newMux(),
	}

	go func() {
		<-ctx.Done()

		err := s.Close()

		slog.WarnContext(ctx, "shutting down API server", "error", err)
	}()

	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}
