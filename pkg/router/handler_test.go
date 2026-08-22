package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helpers ----

// newTestRouter creates a Router wired to the provided fake node servers.
// internalKey is set to "internal" and the router client is pointed at the
// test servers through their real URLs.
func newTestRouter(t *testing.T, nodes []NodeConfig) *Router {
	t.Helper()

	return &Router{
		nodes:        nodes,
		internalKey:  "internal",
		proxyTimeout: 5 * time.Second,
		client:       &http.Client{},
	}
}

// startFakeNode starts an httptest.Server that serves a static NDJSON body on
// GET /kv and echoes PUT/PATCH bodies back with a fixed X-Version header.
func startFakeNode(t *testing.T, nodeID string, keys []string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /kv", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)

		for _, k := range keys {
			_ = enc.Encode(map[string]string{"key": k, "node": nodeID})
		}
	})

	mux.HandleFunc("GET /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Key", r.PathValue("key"))
		w.Header().Set("X-Version", "1")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("value"))
	})

	mux.HandleFunc("PUT /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		w.Header().Set("X-Key", r.PathValue("key"))
		w.Header().Set("X-Version", "1")
		w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Ok"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

// ---- routeKey ----

func TestRouteKey_ReachesCorrectNode(t *testing.T) {
	// Record which node received the request.
	var receivedBy string

	node1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBy = "node-1"

		w.Header().Set("X-Key", r.PathValue("key"))
		w.Header().Set("X-Version", "1")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(node1.Close)

	node2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBy = "node-2"

		w.Header().Set("X-Key", r.PathValue("key"))
		w.Header().Set("X-Version", "1")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(node2.Close)

	nodes := []NodeConfig{
		{ID: "node-1", URL: node1.URL},
		{ID: "node-2", URL: node2.URL},
	}

	r := newTestRouter(t, nodes)

	// Find a key that maps to node-1.
	var targetKey string

	for i := range 1000 {
		k := fmt.Sprintf("probe-%d", i)

		if SelectNode(nodes, k).ID == "node-1" {
			targetKey = k

			break
		}
	}

	require.NotEmpty(t, targetKey, "could not find a key that maps to node-1")

	req := httptest.NewRequest(http.MethodPut, "/kv/"+targetKey, strings.NewReader("v"))
	req.SetPathValue("key", targetKey)

	w := httptest.NewRecorder()

	r.routeKey(w, req)

	assert.Equal(t, "node-1", receivedBy)
}

func TestRouteKey_PreservesStatusCode(t *testing.T) {
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Conflict", http.StatusConflict)
	}))
	t.Cleanup(node.Close)

	r := newTestRouter(t, []NodeConfig{{ID: "n", URL: node.URL}})

	req := httptest.NewRequest(http.MethodPut, "/kv/somekey", strings.NewReader("v"))
	req.SetPathValue("key", "somekey")

	w := httptest.NewRecorder()

	r.routeKey(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestRouteKey_InjectsInternalAPIKey(t *testing.T) {
	var gotKey string

	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")

		w.Header().Set("X-Version", "1")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(node.Close)

	r := &Router{
		nodes:        []NodeConfig{{ID: "n", URL: node.URL}},
		internalKey:  "secret-internal",
		proxyTimeout: 5 * time.Second,
		client:       &http.Client{},
	}

	req := httptest.NewRequest(http.MethodGet, "/kv/k", http.NoBody)
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()

	r.routeKey(w, req)

	assert.Equal(t, "secret-internal", gotKey)
}

// ---- listKeys ----

func TestListKeys_MergesAllNodes(t *testing.T) {
	srv1 := startFakeNode(t, "node-1", []string{"alpha", "beta"})
	srv2 := startFakeNode(t, "node-2", []string{"gamma"})

	r := newTestRouter(t, []NodeConfig{
		{ID: "node-1", URL: srv1.URL},
		{ID: "node-2", URL: srv2.URL},
	})

	req := httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	w := httptest.NewRecorder()

	r.listKeys(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-ndjson", w.Result().Header.Get("Content-Type"))

	var keys []string

	for _, line := range strings.Split(strings.TrimRight(w.Body.String(), "\n"), "\n") {
		if line == "" {
			continue
		}

		var obj map[string]string

		require.NoError(t, json.Unmarshal([]byte(line), &obj))
		keys = append(keys, obj["key"])
	}

	assert.ElementsMatch(t, []string{"alpha", "beta", "gamma"}, keys)
}

func TestListKeys_EmptyStore(t *testing.T) {
	srv := startFakeNode(t, "node-1", nil)
	r := newTestRouter(t, []NodeConfig{{ID: "node-1", URL: srv.URL}})

	req := httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	w := httptest.NewRecorder()

	r.listKeys(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, strings.TrimSpace(w.Body.String()))
}

func TestListKeys_PartialNodeFailure(t *testing.T) {
	// node-1 is healthy, node-2 is down (no server started).
	srv1 := startFakeNode(t, "node-1", []string{"ok-key"})

	r := newTestRouter(t, []NodeConfig{
		{ID: "node-1", URL: srv1.URL},
		{ID: "node-2", URL: "http://127.0.0.1:1"}, // unreachable
	})

	req := httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	w := httptest.NewRecorder()

	r.listKeys(w, req)

	// Still returns 200 with partial results.
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok-key")

	// X-Mimir-Missing-Nodes must be a real response header, set before WriteHeader,
	// so it is visible to clients (not silently dropped after body writes).
	assert.Equal(t, "node-2", w.Result().Header.Get("X-Mimir-Missing-Nodes"))
}

func TestListKeys_NDJSONLineFormat(t *testing.T) {
	srv := startFakeNode(t, "node-1", []string{"my-key"})
	r := newTestRouter(t, []NodeConfig{{ID: "node-1", URL: srv.URL}})

	req := httptest.NewRequest(http.MethodGet, "/kv", http.NoBody)
	w := httptest.NewRecorder()

	r.listKeys(w, req)

	lines := strings.Split(strings.TrimRight(w.Body.String(), "\n"), "\n")

	require.Len(t, lines, 1)
	assert.JSONEq(t, `{"key":"my-key","node":"node-1"}`, lines[0])
}

// ---- healthCheck ----

func TestHealthCheck_AllHealthy(t *testing.T) {
	srv1 := startFakeNode(t, "node-1", nil)
	srv2 := startFakeNode(t, "node-2", nil)

	r := newTestRouter(t, []NodeConfig{
		{ID: "node-1", URL: srv1.URL},
		{ID: "node-2", URL: srv2.URL},
	})

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	w := httptest.NewRecorder()

	r.healthCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Ok", w.Body.String())
}

func TestHealthCheck_OneNodeUnhealthy(t *testing.T) {
	srv1 := startFakeNode(t, "node-1", nil)

	sickNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not ok", http.StatusInternalServerError)
	}))
	t.Cleanup(sickNode.Close)

	r := newTestRouter(t, []NodeConfig{
		{ID: "node-1", URL: srv1.URL},
		{ID: "sick", URL: sickNode.URL},
	})

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	w := httptest.NewRecorder()

	r.healthCheck(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHealthCheck_NodeUnreachable(t *testing.T) {
	r := newTestRouter(t, []NodeConfig{
		{ID: "dead", URL: "http://127.0.0.1:1"},
	})

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	w := httptest.NewRecorder()

	r.healthCheck(w, req)

	// Unreachable node → the HTTP client returns an error, which the handler
	// maps to 503 Service Unavailable (same path as a non-200 response).
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ---- proxy timeout ----

func TestRouteKey_TimeoutOnSlowNode(t *testing.T) {
	// Node accepts the connection but never sends a response, simulating a
	// frozen upstream. The router must abort and return a non-200 status
	// within the proxyTimeout window rather than hanging indefinitely.
	slowNode := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Block until the client context is cancelled (i.e. proxy timeout fires).
		<-r.Context().Done()
	}))
	t.Cleanup(slowNode.Close)

	r := &Router{
		nodes:        []NodeConfig{{ID: "slow", URL: slowNode.URL}},
		internalKey:  "internal",
		proxyTimeout: 50 * time.Millisecond, // very short for test speed
		client:       &http.Client{},
	}

	req := httptest.NewRequest(http.MethodGet, "/kv/mykey", http.NoBody)
	req.SetPathValue("key", "mykey")

	w := httptest.NewRecorder()

	start := time.Now()

	r.routeKey(w, req)

	elapsed := time.Since(start)

	// Must return within a reasonable multiple of the timeout, not hang.
	assert.Less(t, elapsed, 2*time.Second,
		"routeKey should not hang when node is unresponsive")

	// Proxy returns 502 Bad Gateway when the upstream context is cancelled.
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestRouteKey_TimeoutRespected(t *testing.T) {
	// Sanity check: a fast node is unaffected by the proxy timeout.
	fastNode := startFakeNode(t, "fast", nil)

	r := &Router{
		nodes:        []NodeConfig{{ID: "fast", URL: fastNode.URL}},
		internalKey:  "internal",
		proxyTimeout: 5 * time.Second,
		client:       &http.Client{},
	}

	req := httptest.NewRequest(http.MethodGet, "/kv/k", http.NoBody)
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	r.routeKey(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- scanner buffer ----

func TestFetchNodeKeys_LongLineWithinLimit(t *testing.T) {
	// A key that produces a line just under maxScanLineBytes must be returned
	// without error. We use a key of 512 KB (well above the old 64 KB default
	// but well under the 1 MB limit).
	longKey := strings.Repeat("k", 512*1024)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		line := fmt.Sprintf(`{"key":%q,"node":"n1"}`, longKey)

		_, _ = fmt.Fprintln(w, line)
	}))
	t.Cleanup(srv.Close)

	r := newTestRouter(t, []NodeConfig{{ID: "n1", URL: srv.URL}})
	result := r.fetchNodeKeys(t.Context(), NodeConfig{ID: "n1", URL: srv.URL})

	require.NoError(t, result.err)
	require.Len(t, result.lines, 1)
}

func TestFetchNodeKeys_LineExceedsLimit(t *testing.T) {
	// A line over maxScanLineBytes must surface as an error on that node's
	// result rather than silently truncating or dropping the key list.
	tooBigKey := strings.Repeat("x", maxScanLineBytes+1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		line := fmt.Sprintf(`{"key":%q,"node":"n1"}`, tooBigKey)

		_, _ = fmt.Fprintln(w, line)
	}))
	t.Cleanup(srv.Close)

	r := newTestRouter(t, []NodeConfig{{ID: "n1", URL: srv.URL}})
	result := r.fetchNodeKeys(t.Context(), NodeConfig{ID: "n1", URL: srv.URL})

	// Must return an error, not silently succeed with zero lines.
	assert.Error(t, result.err, "oversized line should surface as an error")
}
