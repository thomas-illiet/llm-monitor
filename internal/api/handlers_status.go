package api

import (
	"net/http"
	"time"

	"llmservicemonitor/internal/store"
)

// healthz returns a lightweight process health response.
func (r *Router) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// status returns the compact service status used by external probes.
func (r *Router) status(w http.ResponseWriter, req *http.Request) {
	models, err := r.store.ListModelStates(req.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	providers := r.providerStatuses(req.Context())
	authCheck, httpCheck := aggregateLatestChecks(providers)
	writeJSON(w, http.StatusOK, buildStatus(models, authCheck, httpCheck))
}

// buildStatus derives the top-level OK state from checks and model inventory.
func buildStatus(models []store.ModelState, authCheck, httpCheck *store.CheckRecord) StatusResponse {
	var status StatusResponse
	status.GeneratedAt = time.Now().UTC()
	status.AuthOK = authCheck != nil && authCheck.OK
	status.HTTPOK = httpCheck != nil && httpCheck.OK
	for _, model := range models {
		if model.Excluded || model.Capability == "skip" {
			status.SkippedModels++
		}
		if model.Status == store.ModelStatusInactive || model.Status == "missing" {
			status.InactiveModels++
		}
		if model.Status == store.ModelStatusActive {
			status.ActiveModels++
		}
	}
	status.MissingModels = status.InactiveModels
	status.OK = status.AuthOK && status.HTTPOK && status.InactiveModels == 0
	return status
}
