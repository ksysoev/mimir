package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ksysoev/mimir/pkg/api"
	"github.com/ksysoev/mimir/pkg/repo/inmemory"
	"github.com/spf13/viper"
)

type appConfig struct {
	API  api.Config      `mapstructure:"api"`
	Repo inmemory.Config `mapstructure:"repo"`
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
