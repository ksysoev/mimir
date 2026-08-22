package router

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// Router fans out and proxies HTTP requests to the appropriate storage node.
type Router struct {
	nodes       []NodeConfig
	internalKey string
	client      *http.Client
}

// routeKey proxies GET /kv/{key}, PUT /kv/{key}, and PATCH /kv/{key} to the
// storage node that owns the key, as determined by SelectNode.
func (r *Router) routeKey(w http.ResponseWriter, req *http.Request) {
	key := req.PathValue("key")
	node := SelectNode(r.nodes, key)

	target, err := url.Parse(node.URL)
	if err != nil {
		slog.ErrorContext(req.Context(), "routeKey: invalid node URL",
			"node", node.ID, "url", node.URL, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Override the default Director so we can inject the internal API key and
	// fix the Host header to target the node rather than echoing the client.
	defaultDirector := proxy.Director
	proxy.Director = func(outReq *http.Request) {
		defaultDirector(outReq)
		outReq.Host = target.Host
		outReq.Header.Set("X-API-Key", r.internalKey)
	}

	proxy.ServeHTTP(w, req)
}

// keyLine is the JSON shape written by each storage node's GET /kv endpoint
// and streamed through by the router in the aggregated response.
type keyLine struct {
	Key  string `json:"key"`
	Node string `json:"node"`
}

// nodeKeyResult carries the outcome of a single-node GET /kv call.
type nodeKeyResult struct {
	nodeID string
	lines  []json.RawMessage
	err    error
}

// listKeys fans out GET /kv to all storage nodes in parallel, merges their
// NDJSON responses, and streams the result to the client.
//
// Partial-failure behaviour: nodes that are unreachable or return non-200 are
// skipped and logged. Their IDs are collected in the X-Mimir-Missing-Nodes
// response header so operators can detect degraded list completeness without
// parsing the body. HA is not a goal, so partial results are preferable to
// returning an error when some nodes are available.
func (r *Router) listKeys(w http.ResponseWriter, req *http.Request) {
	results := make([]nodeKeyResult, len(r.nodes))

	var wg sync.WaitGroup

	for i, node := range r.nodes {
		wg.Add(1)

		go func(idx int, n NodeConfig) {
			defer wg.Done()
			results[idx] = r.fetchNodeKeys(req.Context(), n)
		}(i, node)
	}

	wg.Wait()

	var missing []string

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)

	for _, res := range results {
		if res.err != nil {
			slog.ErrorContext(req.Context(), "listKeys: node unavailable",
				"node", res.nodeID, "error", res.err)
			missing = append(missing, res.nodeID)

			continue
		}

		for _, line := range res.lines {
			_, _ = w.Write(line)
			_, _ = w.Write([]byte("\n"))
		}
	}

	if len(missing) > 0 {
		// Best-effort: headers may already be sent, but http.ResponseWriter
		// implementations typically buffer trailers; log regardless.
		w.Header().Set("X-Mimir-Missing-Nodes", strings.Join(missing, ","))
		slog.WarnContext(req.Context(), "listKeys: some nodes unavailable",
			"missing", missing)
	}
}

// fetchNodeKeys calls GET /kv on a single node and returns its NDJSON lines.
func (r *Router) fetchNodeKeys(ctx context.Context, n NodeConfig) nodeKeyResult {

	nodeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, n.URL+"/kv", http.NoBody)
	if err != nil {
		return nodeKeyResult{nodeID: n.ID, err: err}
	}

	nodeReq.Header.Set("X-API-Key", r.internalKey)

	resp, err := r.client.Do(nodeReq)
	if err != nil {
		return nodeKeyResult{nodeID: n.ID, err: err}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nodeKeyResult{nodeID: n.ID, err: fmt.Errorf("node returned status %d", resp.StatusCode)}
	}

	var lines []json.RawMessage

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		raw := make([]byte, len(scanner.Bytes()))
		copy(raw, scanner.Bytes())
		lines = append(lines, raw)
	}

	return nodeKeyResult{nodeID: n.ID, lines: lines, err: scanner.Err()}
}

// healthCheck calls GET /livez on every storage node. Returns 200 only when
// all nodes respond with 200; returns 503 on the first failure.
func (r *Router) healthCheck(w http.ResponseWriter, req *http.Request) {
	for _, n := range r.nodes {
		nodeReq, err := http.NewRequestWithContext(req.Context(), http.MethodGet,
			n.URL+"/livez", http.NoBody)
		if err != nil {
			slog.ErrorContext(req.Context(), "healthCheck: build request failed",
				"node", n.ID, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		resp, err := r.client.Do(nodeReq)
		if err != nil {
			slog.ErrorContext(req.Context(), "healthCheck: node unreachable",
				"node", n.ID, "error", err)
			http.Error(w, "one or more nodes unhealthy", http.StatusServiceUnavailable)

			return
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			slog.ErrorContext(req.Context(), "healthCheck: node unhealthy", "node", n.ID)
			http.Error(w, "one or more nodes unhealthy", http.StatusServiceUnavailable)

			return
		}

		resp.Body.Close()
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("Ok")); err != nil {
		slog.Error("healthCheck: failed to write response", "error", err)
	}
}
