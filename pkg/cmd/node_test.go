package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunNodeCommand_InitLoggerFails(t *testing.T) {
	flags := &cmdFlags{
		LogLevel: "WrongLogLevel",
	}

	err := RunNodeCommand(t.Context(), flags)
	assert.ErrorContains(t, err, "failed to init logger")
}

func TestRunNodeCommand_LoadConfigFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte("invalid config"), 0o600)
	require.NoError(t, err)

	flags := &cmdFlags{
		ConfigPath: configPath,
		LogLevel:   "info",
	}

	err = RunNodeCommand(t.Context(), flags)
	assert.ErrorContains(t, err, "failed to load config:")
}

func TestRunNodeCommand_APIFails(t *testing.T) {
	t.Setenv("API_LISTEN", "WRONG_ADDRESS_TO_LISTEN")
	err := RunNodeCommand(t.Context(), &cmdFlags{LogLevel: "info"})
	assert.ErrorContains(t, err, "failed to run API service:")
}

func TestRunNodeCommand_Success(t *testing.T) {
	t.Setenv("API_LISTEN", ":0")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)

		cancel()
	}()

	err := RunNodeCommand(ctx, &cmdFlags{LogLevel: "info"})
	assert.NoError(t, err, "expected RunCommand to succeed with valid configuration")
}
