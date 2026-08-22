package api

import (
	"net/http"

	"github.com/ksysoev/mimir/pkg/api/middleware"
)

// newMux creates and returns a new HTTP ServeMux with the API's routes registered.
//
// Middleware stack applied to every KV route (outermost → innermost):
//  1. NewReqID    – attaches a unique request ID to the context.
//  2. NewSanitize – enforces max body size and JSON content-type for PUT/PATCH.
//  3. NewAPIKey   – validates X-API-Key when an API key is configured (no-op otherwise).
//
// The /livez health-check route only carries NewReqID so that liveness probes
// work without authentication credentials.
func (a *API) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	withReqID := middleware.NewReqID()
	withSanitize := middleware.NewSanitize(a.config.MaxBodySize)
	withAPIKey := middleware.NewAPIKey(a.config.APIKey)

	mux.Handle("GET /livez", middleware.Use(a.healthCheck, withReqID))
	mux.Handle("GET /kv/{key}", middleware.Use(a.getKey, withReqID, withSanitize, withAPIKey))
	mux.Handle("PUT /kv/{key}", middleware.Use(a.putKey, withReqID, withSanitize, withAPIKey))
	mux.Handle("PATCH /kv/{key}", middleware.Use(a.patchKey, withReqID, withSanitize, withAPIKey))

	return mux
}
