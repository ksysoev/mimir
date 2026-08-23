package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NilConfig(t *testing.T) {
	_, err := New(nil, NewMockService(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config must not be nil")
}

func TestNew_ValidConfig(t *testing.T) {
	cfg := &Config{Listen: ":8080"}
	svc := NewMockService(t)
	api, err := New(cfg, svc)

	assert.NoError(t, err)
	assert.NotNil(t, api)
}

func TestNew_InvalidConfig(t *testing.T) {
	cfg := &Config{Listen: ""}
	svc := NewMockService(t)
	_, err := New(cfg, svc)

	assert.Error(t, err)
}

func TestAPI_Run_StartAndShutdown(t *testing.T) {
	cfg := &Config{Listen: "127.0.0.1:0"}
	svc := NewMockService(t)
	api, err := New(cfg, svc)

	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err = api.Run(ctx)

	assert.NoError(t, err)
}
