package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ksysoev/mimir/pkg/api"
	"github.com/ksysoev/mimir/pkg/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	const validConfig = `
api:
  listen: ":8082"
`

	tests := []struct {
		envVars      map[string]string
		expectConfig *appConfig
		name         string
		configData   string
		expectError  bool
	}{
		{
			name:        "valid config file",
			envVars:     nil,
			expectError: false,
			configData:  validConfig,
			expectConfig: &appConfig{
				API: api.Config{
					Listen: ":8082",
				},
			},
		},
		{
			name:        "missing config file",
			envVars:     nil,
			expectError: true,
		},
		{
			name:        "unparseable config file",
			envVars:     nil,
			expectError: true,
			configData:  `invalid yaml`,
		},
		{
			name: "valid config with environment overrides",
			envVars: map[string]string{
				"API_LISTEN": ":8083",
			},
			expectError: false,
			configData:  validConfig,
			expectConfig: &appConfig{
				API: api.Config{
					Listen: ":8083",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if tt.configData != "" {
				err := os.WriteFile(configPath, []byte(tt.configData), 0o600)
				require.NoError(t, err)
			}

			if tt.envVars != nil {
				for key, value := range tt.envVars {
					_ = os.Setenv(key, value)

					t.Cleanup(func() {
						_ = os.Unsetenv(key)
					})
				}
			}

			arg := &cmdFlags{ConfigPath: configPath}
			cfg, err := loadConfig(arg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectConfig, cfg)
			}
		})
	}
}

func TestLoadRouterConfig(t *testing.T) {
	const validConfig = `
router:
  listen: ":7000"
  key: "file-key"
  internal_key: "file-internal"
  nodes:
    - id: "node-1"
      url: "http://localhost:7001"
    - id: "node-2"
      url: "http://localhost:7002"
`

	tests := []struct {
		envVars      map[string]string
		expectConfig *routerConfig
		name         string
		expectErrMsg string
		configData   string
		expectError  bool
	}{
		{
			name:        "valid config file",
			configData:  validConfig,
			expectError: false,
			expectConfig: &routerConfig{
				Router: router.Config{
					Listen:      ":7000",
					Key:         "file-key",
					InternalKey: "file-internal",
					Nodes: []router.NodeConfig{
						{ID: "node-1", URL: "http://localhost:7001"},
						{ID: "node-2", URL: "http://localhost:7002"},
					},
				},
			},
		},
		{
			name:        "missing config file returns error",
			expectError: true,
		},
		{
			name:         "unparseable config file returns error",
			configData:   `invalid yaml: [`,
			expectError:  true,
			expectErrMsg: "failed to read config",
		},
		{
			name:       "scalar env vars override file values",
			configData: validConfig,
			envVars: map[string]string{
				"ROUTER_LISTEN": ":9000",
				"ROUTER_KEY":    "env-key",
			},
			expectError: false,
			expectConfig: &routerConfig{
				Router: router.Config{
					Listen:      ":9000",
					Key:         "env-key",
					InternalKey: "file-internal",
					Nodes: []router.NodeConfig{
						{ID: "node-1", URL: "http://localhost:7001"},
						{ID: "node-2", URL: "http://localhost:7002"},
					},
				},
			},
		},
		{
			// Viper splits env var values on commas before mapstructure sees them,
			// shredding a JSON array into unusable strings. loadRouterConfig reads
			// ROUTER_NODES directly and parses it as JSON to work around this.
			name:       "ROUTER_NODES env var parsed as JSON array",
			configData: validConfig,
			envVars: map[string]string{
				"ROUTER_NODES": `[{"id":"env-node-1","url":"http://envhost:8001"},{"id":"env-node-2","url":"http://envhost:8002"}]`,
			},
			expectError: false,
			expectConfig: &routerConfig{
				Router: router.Config{
					Listen:      ":7000",
					Key:         "file-key",
					InternalKey: "file-internal",
					Nodes: []router.NodeConfig{
						{ID: "env-node-1", URL: "http://envhost:8001"},
						{ID: "env-node-2", URL: "http://envhost:8002"},
					},
				},
			},
		},
		{
			name: "ROUTER_NODES invalid JSON returns error",
			envVars: map[string]string{
				"ROUTER_NODES": `not-json`,
			},
			expectError:  true,
			expectErrMsg: "failed to parse ROUTER_NODES",
		},
		{
			// Env-only config: no file, nodes supplied entirely via ROUTER_NODES.
			name: "no config file, nodes from env only",
			envVars: map[string]string{
				"ROUTER_LISTEN": ":7000",
				"ROUTER_NODES":  `[{"id":"n1","url":"http://n1:7001"}]`,
			},
			expectError: false,
			expectConfig: &routerConfig{
				Router: router.Config{
					Listen: ":7000",
					Nodes:  []router.NodeConfig{{ID: "n1", URL: "http://n1:7001"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			var arg *cmdFlags

			if tt.configData != "" {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "router.yaml")

				err := os.WriteFile(configPath, []byte(tt.configData), 0o600)
				require.NoError(t, err)

				arg = &cmdFlags{ConfigPath: configPath}
			} else {
				// No config file: use a non-existent path only when the test
				// explicitly expects a missing-file error; otherwise pass empty
				// so loadRouterConfig skips file loading entirely.
				if tt.expectError && tt.expectErrMsg == "" {
					arg = &cmdFlags{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}
				} else {
					arg = &cmdFlags{}
				}
			}

			cfg, err := loadRouterConfig(arg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)

				if tt.expectErrMsg != "" {
					assert.ErrorContains(t, err, tt.expectErrMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectConfig, cfg)
			}
		})
	}
}
