package cmd

import (
	"context"
	"fmt"

	"github.com/ksysoev/mimir/pkg/router"
)

// RunRouterCommand initializes the logger, loads configuration, and starts the
// cluster router that distributes key-value requests across storage nodes.
func RunRouterCommand(ctx context.Context, flags *cmdFlags) error {
	if err := initLogger(flags); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadRouterConfig(flags)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	r, err := router.New(&cfg.Router)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	if err = r.Run(ctx, &cfg.Router); err != nil {
		return fmt.Errorf("failed to run router: %w", err)
	}

	return nil
}
