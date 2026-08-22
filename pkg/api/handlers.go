package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ksysoev/mimir/pkg/core"
)

const (
	// headerKey is the response header that carries the stored key name.
	headerKey = "X-Key"
	// headerVersion is the response header that carries the item version.
	headerVersion = "X-Version"
)

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
// The response body contains the raw stored value.
// X-Key and X-Version response headers carry metadata.
func (a *API) getKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	item, err := a.svc.GetKey(r.Context(), key)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		slog.ErrorContext(r.Context(), "getKey failed", "key", key, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	writeItem(w, http.StatusOK, item)
}

// putKey handles PUT /kv/{key}.
// Accepts any content type; the Content-Type header is preserved and returned on GET.
// The optional ifVersion query parameter sets a CAS guard (Version > 0); omitting it
// performs an unconditional write (Version == 0).
func (a *API) putKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	body, contentType, ok := readBody(w, r)
	if !ok {
		return
	}

	ifVersion, ok := parseIfVersion(w, r)
	if !ok {
		return
	}

	item, err := a.svc.PutKey(r.Context(), core.Item{
		Key:         key,
		Value:       body,
		ContentType: contentType,
		Version:     ifVersion,
	})
	if err != nil {
		switch {
		case errors.Is(err, core.ErrInvalidPayload):
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		case errors.Is(err, core.ErrVersionMismatch):
			http.Error(w, "Conflict", http.StatusConflict)
		default:
			slog.ErrorContext(r.Context(), "putKey failed", "key", key, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}

		return
	}

	writeItem(w, http.StatusOK, item)
}

// patchKey handles PATCH /kv/{key}.
// Performs an early Content-Type header check before reading the body to avoid
// buffering large payloads that will be rejected. Core enforces the same rule
// authoritatively for non-HTTP callers.
// The optional ifVersion query parameter sets a CAS guard (Version > 0); omitting it
// performs an unconditional patch (Version == 0).
func (a *API) patchKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	// Early header-only check: avoids reading a large body that will be rejected.
	// Core re-validates this for callers that bypass the HTTP layer.
	if !(core.Item{ContentType: r.Header.Get("Content-Type")}).IsJSON() {
		http.Error(w, "Patch requires Content-Type: application/json", http.StatusUnsupportedMediaType)
		return
	}

	body, contentType, ok := readBody(w, r)
	if !ok {
		return
	}

	ifVersion, ok := parseIfVersion(w, r)
	if !ok {
		return
	}

	item, err := a.svc.PatchKey(r.Context(), core.Item{
		Key:         key,
		Value:       body,
		ContentType: contentType,
		Version:     ifVersion,
	})
	if err != nil {
		switch {
		case errors.Is(err, core.ErrUnsupportedContentType):
			http.Error(w, "Patch requires Content-Type: application/json", http.StatusUnsupportedMediaType)
		case errors.Is(err, core.ErrInvalidPayload):
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		case errors.Is(err, core.ErrVersionMismatch):
			http.Error(w, "Conflict", http.StatusConflict)
		default:
			slog.ErrorContext(r.Context(), "patchKey failed", "key", key, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}

		return
	}

	writeItem(w, http.StatusOK, item)
}

// writeItem writes the raw item value as the response body.
// Metadata (key name, version) is conveyed via X-Key and X-Version headers.
func writeItem(w http.ResponseWriter, status int, item core.Item) {
	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set(headerKey, item.Key)
	w.Header().Set(headerVersion, strconv.FormatUint(item.Version, 10))
	w.WriteHeader(status)

	if _, err := w.Write(item.Value); err != nil {
		slog.Error("Failed to write item response", "error", err)
	}
}

// readBody reads the full request body and the Content-Type header.
// Falls back to kv.DefaultContentType when Content-Type is absent.
// Returns (nil, "", false) and writes a 400 response on read failure.
func readBody(w http.ResponseWriter, r *http.Request) (body []byte, contentType string, ok bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return nil, "", false
	}

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = core.DefaultContentType
	}

	return body, ct, true
}

// parseIfVersion parses the optional ifVersion query parameter.
// Returns (0, true) when the parameter is absent — 0 signals an unconditional write.
// Returns (0, false) and writes a 400 response when the value is malformed or zero
// (valid stored versions start at 1, so ifVersion=0 would be ambiguous with "absent").
func parseIfVersion(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	raw := r.URL.Query().Get("ifVersion")
	if raw == "" {
		return 0, true
	}

	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ifVersion parameter", http.StatusBadRequest)
		return 0, false
	}

	if v == 0 {
		http.Error(w, "ifVersion must be greater than 0", http.StatusBadRequest)
		return 0, false
	}

	return v, true
}
