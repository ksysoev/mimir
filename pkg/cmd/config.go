package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ksysoev/mimir/pkg/api"
	"github.com/ksysoev/mimir/pkg/repo/inmemory"
	"github.com/ksysoev/mimir/pkg/router"
	"github.com/spf13/viper"
)

type appConfig struct {
	API  api.Config      `mapstructure:"api"`
	Repo inmemory.Config `mapstructure:"repo"`
}

type routerConfig struct {
	Router router.Config `mapstructure:"router"`
}

// loadConfig loads the application configuration from the specified file path and environment variables.
func loadConfig(flags *cmdFlags) (*appConfig, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())

	if flags.ConfigPath != "" {
		v.SetConfigFile(flags.ConfigPath)

		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var cfg appConfig

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	slog.Debug("Config loaded",
		slog.String("listen", cfg.API.Listen),
		slog.Int("max_keys", cfg.Repo.MaxKeys),
	)

	return &cfg, nil
}

// loadRouterConfig loads router configuration from the file (if specified) and
// environment variables. The env prefix mirrors the field path, e.g.:
//
//	ROUTER_LISTEN, ROUTER_KEY, ROUTER_INTERNAL_KEY.
//
// ROUTER_NODES is a special case: Viper cannot decode a JSON array of structs
// from an env var (it splits on commas and loses the object structure).
// loadRouterConfig reads ROUTER_NODES directly and parses it as a JSON array:
//
//	ROUTER_NODES='[{"id":"node-1","url":"http://node1:7001"},{"id":"node-2","url":"http://node2:7002"}]'
func loadRouterConfig(flags *cmdFlags) (*routerConfig, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())

	if flags.ConfigPath != "" {
		v.SetConfigFile(flags.ConfigPath)

		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// ROUTER_NODES requires special handling: Viper splits env var values on
	// commas before mapstructure sees them, shredding a JSON array of structs
	// into individual strings that cannot be decoded into []NodeConfig.
	// We capture the raw value, hide it from Viper's unmarshal pass, then parse
	// it ourselves as JSON afterwards.
	nodesJSON := os.Getenv("ROUTER_NODES")

	if nodesJSON != "" {
		os.Unsetenv("ROUTER_NODES")

		// Restore the env var after Viper's unmarshal so other code sharing
		// this process (e.g. tests) still sees the original value.
		defer os.Setenv("ROUTER_NODES", nodesJSON)
	}

	var cfg routerConfig

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Parse ROUTER_NODES as a JSON array, overriding anything from the file.
	if nodesJSON != "" {
		var nodes []router.NodeConfig

		if err := json.Unmarshal([]byte(nodesJSON), &nodes); err != nil {
			return nil, fmt.Errorf("failed to parse ROUTER_NODES: %w", err)
		}

		cfg.Router.Nodes = nodes
	}

	slog.Debug("Router config loaded",
		slog.String("listen", cfg.Router.Listen),
		slog.Int("nodes", len(cfg.Router.Nodes)),
	)

	return &cfg, nil
}
