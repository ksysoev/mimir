package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ksysoev/mimir/pkg/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NoNodes(t *testing.T) {
	_, err := New(&Config{Listen: ":0"})
	assert.ErrorContains(t, err, "at least one node")
}

func TestNew_NoListen(t *testing.T) {
	_, err := New(&Config{Nodes: []NodeConfig{{ID: "n", URL: "http://x"}}})
	assert.ErrorContains(t, err, "listen address")
}

func TestNew_Valid(t *testing.T) {
	r, err := New(&Config{
		Listen: ":0",
		Nodes:  []NodeConfig{{ID: "n", URL: "http://x"}},
	})
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestRouter_Run_Lifecycle(t *testing.T) {
	srv1 := startFakeNode(t, "node-1", nil)

	cfg := &Config{
		Listen: "127.0.0.1:0",
		Nodes:  []NodeConfig{{ID: "node-1", URL: srv1.URL}},
	}

	r, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)

	go func() {
		done <- r.Run(ctx, cfg)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("router did not shut down within timeout")
	}
}

// TestRouter_Mux_ClientAPIKeyEnforced verifies that the client-facing Key
// is applied on /kv routes by exercising the mux directly (no network).
func TestRouter_Mux_ClientAPIKeyEnforced(t *testing.T) {
	srv1 := startFakeNode(t, "node-1", []string{"k"})

	cfg := Config{
		Key:   "client-secret",
		Nodes: []NodeConfig{{ID: "node-1", URL: srv1.URL}},
	}

	r := &Router{
		nodes:       cfg.Nodes,
		internalKey: cfg.InternalKey,
		client:      &http.Client{},
	}

	withReqID := middleware.NewReqID()
	withAPIKey := middleware.NewAPIKey(cfg.Key)

	mux := http.NewServeMux()
	mux.Handle("GET /kv", middleware.Use(r.listKeys, withReqID, withAPIKey))

	// Without API key → 401.
	req := httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// With correct API key → 200.
	req = httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	req.Header.Set("X-API-Key", "client-secret")

	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouter_Mux_LivezNoAuthRequired(t *testing.T) {
	srv1 := startFakeNode(t, "node-1", nil)

	cfg := Config{
		Key:   "secret",
		Nodes: []NodeConfig{{ID: "node-1", URL: srv1.URL}},
	}

	r := &Router{
		nodes:  cfg.Nodes,
		client: &http.Client{},
	}

	withReqID := middleware.NewReqID()

	mux := http.NewServeMux()
	mux.Handle("GET /livez", middleware.Use(r.healthCheck, withReqID))

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
