package cmd

import (
	"fmt"
	"log/slog"
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

	slog.Debug("Config loaded", slog.Any("config", cfg))

	return &cfg, nil
}

// loadRouterConfig loads router configuration from the file (if specified) and
// environment variables. The env prefix mirrors the field path, e.g.:
//
//	ROUTER_LISTEN, ROUTER_KEY, ROUTER_INTERNAL_KEY, ROUTER_NODES.
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

	var cfg routerConfig

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	slog.Debug("Router config loaded", slog.Any("config", cfg))

	return &cfg, nil
}
