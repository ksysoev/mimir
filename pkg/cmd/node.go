package cmd

import (
	"context"
	"fmt"

	"github.com/ksysoev/mimir/pkg/api"
	"github.com/ksysoev/mimir/pkg/core"
	"github.com/ksysoev/mimir/pkg/repo/inmemory"
)

// RunNodeCommand initializes the logger, loads configuration, wires dependencies,
// and starts the storage node API server.
func RunNodeCommand(ctx context.Context, flags *cmdFlags) error {
	if err := initLogger(flags); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(flags)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store := inmemory.NewStore(cfg.Repo)
	svc := core.New(store)

	apiSvc, err := api.New(cfg.API, svc)
	if err != nil {
		return fmt.Errorf("failed to create API service: %w", err)
	}

	if err = apiSvc.Run(ctx); err != nil {
		return fmt.Errorf("failed to run API service: %w", err)
	}

	return nil
}
