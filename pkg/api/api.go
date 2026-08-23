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
	startTime time.Time
	svc       Service
	config    Config
}

type Config struct {
	Listen      string `mapstructure:"listen"`
	Key         string `mapstructure:"key"`
	NodeID      string `mapstructure:"node_id"`
	Version     string `mapstructure:"version"`
	AppName     string `mapstructure:"app_name"`
	MaxBodySize int64  `mapstructure:"max_body_size"`
}

type Service interface {
	CheckHealth(ctx context.Context) error
	GetKey(ctx context.Context, key string) (core.Item, error)
	PutKey(ctx context.Context, item core.Item) (core.Item, error)
	PatchKey(ctx context.Context, item core.Item) (core.Item, error)
	ListKeys(ctx context.Context, nodeID string) []core.KeyEntry
}

// New creates a new API instance with the provided configuration and service.
// It validates the configuration and returns an error if the listen address is not specified.
func New(cfg *Config, svc Service) (*API, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}

	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address must be specified")
	}

	api := &API{
		config:    *cfg,
		svc:       svc,
		startTime: time.Now(),
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
