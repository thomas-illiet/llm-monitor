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
	providers := r.providerStatuses(ctx)
	authCheck, httpCheck := aggregateLatestChecks(providers)
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
		Providers:          providers,
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
		nextCheck, ok := nextChecks[store.ModelIdentityKey(models[i].ProviderID, models[i].ModelID)]
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
		SiteName:  r.cfg.Dashboard.SiteName,
		SiteURL:   r.cfg.Dashboard.SiteURL,
		Providers: r.providerRuntimeConfigs(),
		Retention: RetentionRuntimeConfig{
			HistorySeconds: int64(history / time.Second),
		},
	}
}

func (r *Router) providerStatuses(ctx context.Context) []ProviderStatusResponse {
	statuses := make([]ProviderStatusResponse, 0, len(r.cfg.Providers))
	for _, provider := range r.cfg.Providers {
		authCheck, _ := r.store.LatestAuthCheck(ctx, provider.ID)
		httpCheck, _ := r.store.LatestHTTPCheck(ctx, provider.ID)
		statuses = append(statuses, ProviderStatusResponse{
			ID:   provider.ID,
			Name: provider.Name,
			Auth: authCheck,
			HTTP: httpCheck,
		})
	}
	return statuses
}

func (r *Router) providerRuntimeConfigs() []ProviderRuntimeConfig {
	providers := make([]ProviderRuntimeConfig, 0, len(r.cfg.Providers))
	for _, provider := range r.cfg.Providers {
		providers = append(providers, ProviderRuntimeConfig{ID: provider.ID, Name: provider.Name})
	}
	return providers
}

func aggregateLatestChecks(providers []ProviderStatusResponse) (*store.CheckRecord, *store.CheckRecord) {
	var authCheck *store.CheckRecord
	var httpCheck *store.CheckRecord
	for _, provider := range providers {
		authCheck = aggregateCheck(authCheck, provider.Auth)
		httpCheck = aggregateCheck(httpCheck, provider.HTTP)
	}
	return authCheck, httpCheck
}

func aggregateCheck(current, next *store.CheckRecord) *store.CheckRecord {
	if next == nil {
		return current
	}
	if current == nil {
		return next
	}
	if !next.OK && current.OK {
		return next
	}
	if next.At.After(current.At) {
		return next
	}
	return current
}
