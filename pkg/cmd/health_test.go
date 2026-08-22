package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHealthCmd(t *testing.T) {
	cmd := newHealthCmd()

	assert.Equal(t, "health", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	urlFlag := cmd.Flags().Lookup("url")
	require.NotNil(t, urlFlag)
	assert.Equal(t, "http://localhost:7000", urlFlag.DefValue)
}

func TestRunHealthCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/livez", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := runHealthCheck(context.Background(), srv.URL)
	require.NoError(t, err)
}

func TestRunHealthCheck_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := runHealthCheck(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestRunHealthCheck_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	url := srv.URL
	srv.Close() // close before calling so the connection is refused

	err := runHealthCheck(context.Background(), url)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}
