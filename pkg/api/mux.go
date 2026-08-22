package api

import (
	"net/http"

	"github.com/ksysoev/mimir/pkg/api/middleware"
)

// newMux creates and returns a new HTTP ServeMux with the API's routes registered.
//
// Middleware stack applied to every KV route (outermost → innermost):
//  1. NewReqID    – attaches a unique request ID to the context.
//  2. NewAPIKey   – validates X-API-Key when configured; all unauthenticated
//     requests receive 401 before any payload inspection occurs.
//  3. NewSanitize – enforces max body size and JSON content-type for PUT/PATCH.
//
// The /livez health-check route only carries NewReqID so that liveness probes
// work without authentication credentials.
func (a *API) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	withReqID := middleware.NewReqID()
	withAPIKey := middleware.NewAPIKey(a.config.Key)
	withSanitize := middleware.NewSanitize(a.config.MaxBodySize)

	mux.Handle("GET /livez", middleware.Use(a.healthCheck, withReqID))
	mux.Handle("GET /kv/{key}", middleware.Use(a.getKey, withReqID, withAPIKey, withSanitize))
	mux.Handle("PUT /kv/{key}", middleware.Use(a.putKey, withReqID, withAPIKey, withSanitize))
	mux.Handle("PATCH /kv/{key}", middleware.Use(a.patchKey, withReqID, withAPIKey, withSanitize))

	return mux
}
