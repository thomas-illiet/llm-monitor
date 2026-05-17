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
	authCheck, _ := r.store.LatestAuthCheck(req.Context())
	httpCheck, _ := r.store.LatestHTTPCheck(req.Context())
	writeJSON(w, http.StatusOK, buildStatus(models, authCheck, httpCheck))
}

// dashboard returns all data required by the single-page dashboard.
func (r *Router) dashboard(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	now := time.Now().UTC()
	window := parseDashboardWindow(req.URL.Query(), r.cfg.Dashboard.DefaultWindow.Duration)
	since := now.Add(-window)
	models, err := r.store.ListModelStates(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	authCheck, _ := r.store.LatestAuthCheck(ctx)
	httpCheck, _ := r.store.LatestHTTPCheck(ctx)
	slo := r.sloThresholds()
	kpis, err := r.store.KPISummary(ctx, since, slo)
	if err != nil {
		writeError(w, err)
		return
	}
	events, err := r.store.RecentModelEvents(ctx, 30)
	if err != nil {
		writeError(w, err)
		return
	}
	runs, err := r.store.RecentRuns(ctx, 30)
	if err != nil {
		writeError(w, err)
		return
	}
	alerts, err := r.store.RecentAlerts(ctx, 20)
	if err != nil {
		writeError(w, err)
		return
	}
	charts := make([]ChartResponse, 0, len(dashboardCharts))
	for _, chartCfg := range dashboardCharts {
		charts = append(charts, r.buildChart(ctx, chartCfg, since, now, window))
	}
	response := DashboardResponse{
		GeneratedAt:        now,
		Status:             buildStatus(models, authCheck, httpCheck),
		KPIs:               kpis,
		SLO:                slo,
		Charts:             charts,
		ModelStatusHistory: r.buildModelStatusHistory(ctx, models, since, now, window),
		Models:             models,
		Events:             events,
		Runs:               runs,
		Alerts:             alerts,
		Auth:               authCheck,
		HTTP:               httpCheck,
	}
	writeJSON(w, http.StatusOK, response)
}

// modelDashboard returns model-scoped KPI, chart, and run telemetry.
func (r *Router) modelDashboard(w http.ResponseWriter, req *http.Request) {
	query, errMessage := parseModelDashboardQuery(req.URL.Query(), r.cfg.Dashboard.DefaultWindow.Duration)
	if errMessage != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMessage})
		return
	}
	ctx := req.Context()
	now := time.Now().UTC()
	since := now.Add(-query.Window)
	models, err := r.store.ListModelStates(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	model := findModelState(models, query.ModelID)
	if model == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "model not found"})
		return
	}
	slo := r.sloThresholds()
	kpis, err := r.store.KPISummaryForModel(ctx, query.ModelID, since, slo)
	if err != nil {
		writeError(w, err)
		return
	}
	runs, err := r.store.RecentRunsForModel(ctx, query.ModelID, since, 30)
	if err != nil {
		writeError(w, err)
		return
	}
	charts := make([]ChartResponse, 0, len(modelDashboardCharts))
	for _, chartCfg := range modelDashboardCharts {
		charts = append(charts, r.buildModelChart(ctx, chartCfg, query.ModelID, since, now, query.Window))
	}
	writeJSON(w, http.StatusOK, ModelDashboardResponse{
		GeneratedAt: now,
		Model:       *model,
		KPIs:        kpis,
		SLO:         slo,
		Charts:      charts,
		Runs:        runs,
	})
}

// modelEvents returns a model-scoped event timeline for dashboard modals.
func (r *Router) modelEvents(w http.ResponseWriter, req *http.Request) {
	query, errMessage := parseModelEventsQuery(req.URL.Query())
	if errMessage != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMessage})
		return
	}
	page, err := r.store.ListModelEvents(req.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ModelEventsResponse{
		ModelID: query.ModelID,
		Events:  page.Events,
		Total:   page.Total,
		Limit:   query.Limit,
		Offset:  query.Offset,
		Filters: page.Filters,
	})
}

// findModelState returns the current state for one model.
func findModelState(models []store.ModelState, modelID string) *store.ModelState {
	for i := range models {
		if models[i].ModelID == modelID {
			return &models[i]
		}
	}
	return nil
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
		if model.Status == "missing" {
			status.MissingModels++
		}
		if model.Status == "active" {
			status.ActiveModels++
		}
	}
	status.OK = status.AuthOK && status.HTTPOK && status.MissingModels == 0
	return status
}
