package api

import (
	"context"
	"net/http"
	"time"

	"llmservicemonitor/internal/store"
)

// dashboard returns all data required by the single-page dashboard.
func (r *Router) dashboard(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	now := time.Now().UTC()
	window := capDashboardWindow(parseDashboardWindow(req.URL.Query(), r.cfg.Dashboard.DefaultWindow.Duration), r.cfg.Retention.History.Duration)
	since := now.Add(-window)
	models, err := r.store.ListModelStates(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	models = r.attachModelNextChecks(ctx, models)
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
		Config:             r.runtimeConfig(),
	}
	writeJSON(w, http.StatusOK, response)
}

func (r *Router) attachModelNextChecks(ctx context.Context, models []store.ModelState) []store.ModelState {
	reader, ok := r.taskQueue.(modelRunScheduleReader)
	if !ok {
		return models
	}
	nextChecks, err := reader.ScheduledModelRuns(ctx)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("read model run schedules", "error", err)
		}
		return models
	}
	for i := range models {
		nextCheck, ok := nextChecks[models[i].ModelID]
		if !ok {
			continue
		}
		models[i].NextCheckAt = &nextCheck
	}
	return models
}

// runtimeConfig returns non-secret configuration values consumed by the SPA.
func (r *Router) runtimeConfig() RuntimeConfig {
	history := r.cfg.Retention.History.Duration
	if history <= 0 {
		history = 0
	}
	return RuntimeConfig{
		SiteName: r.cfg.Dashboard.SiteName,
		SiteURL:  r.cfg.Dashboard.SiteURL,
		Retention: RetentionRuntimeConfig{
			HistorySeconds: int64(history / time.Second),
		},
	}
}
