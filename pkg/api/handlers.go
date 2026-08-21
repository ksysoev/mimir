package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ksysoev/mimir/pkg/repo/kv"
)

// itemResponse is the JSON envelope returned for all KV operations.
type itemResponse struct {
	Key     string          `json:"key"`
	Value   json.RawMessage `json:"value"`
	Version int64           `json:"version"`
}

// healthCheck verifies the health of the service.
func (a *API) healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := a.svc.CheckHealth(r.Context()); err != nil {
		slog.ErrorContext(r.Context(), "Health check failed", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("Ok")); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", "error", err)
	}
}

// getKey handles GET /kv/{key}.
func (a *API) getKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	item, err := a.svc.GetKey(r.Context(), key)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		slog.ErrorContext(r.Context(), "getKey failed", "key", key, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	writeJSON(w, http.StatusOK, itemResponse{Key: item.Key, Value: item.Value, Version: item.Version})
}

// putKey handles PUT /kv/{key}.
func (a *API) putKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	value, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	ifVersion, ok := parseIfVersion(w, r)
	if !ok {
		return
	}

	item, err := a.svc.PutKey(r.Context(), key, value, ifVersion)
	if err != nil {
		if errors.Is(err, kv.ErrVersionMismatch) {
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}

		slog.ErrorContext(r.Context(), "putKey failed", "key", key, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	writeJSON(w, http.StatusOK, itemResponse{Key: item.Key, Value: item.Value, Version: item.Version})
}

// patchKey handles PATCH /kv/{key}.
func (a *API) patchKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	delta, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	ifVersion, ok := parseIfVersion(w, r)
	if !ok {
		return
	}

	item, err := a.svc.PatchKey(r.Context(), key, delta, ifVersion)
	if err != nil {
		if errors.Is(err, kv.ErrVersionMismatch) {
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}

		slog.ErrorContext(r.Context(), "patchKey failed", "key", key, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	writeJSON(w, http.StatusOK, itemResponse{Key: item.Key, Value: item.Value, Version: item.Version})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

// readJSONBody reads the request body and validates it as JSON.
// It writes an appropriate error response and returns false on failure.
func readJSONBody(w http.ResponseWriter, r *http.Request) (json.RawMessage, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return nil, false
	}

	if !json.Valid(body) {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return nil, false
	}

	return json.RawMessage(body), true
}

// parseIfVersion parses the optional ifVersion query parameter.
// Returns (nil, true) when the parameter is absent.
// Returns (nil, false) and writes a 400 response when the value is malformed.
func parseIfVersion(w http.ResponseWriter, r *http.Request) (*int64, bool) {
	raw := r.URL.Query().Get("ifVersion")
	if raw == "" {
		return nil, true
	}

	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ifVersion parameter", http.StatusBadRequest)
		return nil, false
	}

	return &v, true
}
