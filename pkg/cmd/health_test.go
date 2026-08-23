package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ksysoev/mimir/pkg/livez"
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

// ---- runHealthCheck ----

func TestRunHealthCheck_OK_JSON_Node(t *testing.T) {
	payload := livez.Response{
		App:       "mimir",
		Version:   "v1.0.0",
		Component: "node",
		Node:      "node-1",
		Uptime:    "1m30s",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/livez", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	err := runHealthCheck(context.Background(), srv.URL)
	require.NoError(t, err)
}

func TestRunHealthCheck_OK_JSON_Router(t *testing.T) {
	// Router responses have no Node field (omitempty).
	payload := livez.Response{
		App:       "mimir",
		Version:   "v1.0.0",
		Component: "router",
		Uptime:    "5m0s",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	err := runHealthCheck(context.Background(), srv.URL)
	require.NoError(t, err)
}

func TestRunHealthCheck_MalformedJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	err := runHealthCheck(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode health response")
}

func TestRunHealthCheck_OK_PlainTextFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Ok"))
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

// ---- printLivezResponse ----

func TestPrintLivezResponse_Node(t *testing.T) {
	var buf bytes.Buffer

	printLivezResponse(&buf, &livez.Response{
		App:       "mimir",
		Version:   "v2.0.0",
		Component: "node",
		Node:      "node-3",
		Uptime:    "3h15m0s",
	})

	out := buf.String()
	assert.Contains(t, out, "Status:    OK")
	assert.Contains(t, out, "App:       mimir")
	assert.Contains(t, out, "Version:   v2.0.0")
	assert.Contains(t, out, "Component: node")
	assert.Contains(t, out, "Node:      node-3")
	assert.Contains(t, out, "Uptime:    3h15m0s")
}

func TestPrintLivezResponse_Router_NoNodeLine(t *testing.T) {
	var buf bytes.Buffer

	printLivezResponse(&buf, &livez.Response{
		App:       "mimir",
		Version:   "v2.0.0",
		Component: "router",
		Uptime:    "10m0s",
	})

	out := buf.String()
	assert.Contains(t, out, "Status:    OK")
	assert.Contains(t, out, "Component: router")
	assert.Contains(t, out, "Uptime:    10m0s")
	assert.NotContains(t, out, "Node:")
}
