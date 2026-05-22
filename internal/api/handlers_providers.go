package api

import (
	"net/http"
	"time"
)

// providers returns configured non-secret provider labels and latest checks.
func (r *Router) providers(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC(),
		"providers":    r.providerStatuses(req.Context()),
	})
}
