package api

import (
	"net/http"

	"github.com/ksysoev/mimir/pkg/api/middleware"
)

// newMux creates and returns a new HTTP ServeMux with the API's routes registered.
func (a *API) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	withReqID := middleware.NewReqID()

	mux.Handle("GET /livez", middleware.Use(a.healthCheck, withReqID))
	mux.Handle("GET /kv/{key}", middleware.Use(a.getKey, withReqID))
	mux.Handle("PUT /kv/{key}", middleware.Use(a.putKey, withReqID))
	mux.Handle("PATCH /kv/{key}", middleware.Use(a.patchKey, withReqID))

	return mux
}
