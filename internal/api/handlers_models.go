package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"llmservicemonitor/internal/store"

	"github.com/jackc/pgx/v5"
)

// modelDashboard returns model-scoped KPI, chart, and run telemetry.
func (r *Router) modelDashboard(w http.ResponseWriter, req *http.Request) {
	query, errMessage := parseModelDashboardQuery(req.URL.Query(), r.cfg.Dashboard.DefaultWindow.Duration)
	if errMessage != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMessage})
		return
	}
	ctx := req.Context()
	now := time.Now().UTC()
	query.Window = capDashboardWindow(query.Window, r.cfg.Retention.History.Duration)
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
	chartConfigs := modelDashboardChartConfigs(model.Capability)
	charts := make([]ChartResponse, 0, len(chartConfigs))
	for _, chartCfg := range chartConfigs {
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

// modelDetails returns provider metadata captured from the latest model inventory.
func (r *Router) modelDetails(w http.ResponseWriter, req *http.Request) {
	modelID := strings.TrimSpace(req.PathValue("model_id"))
	if modelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_id is required"})
		return
	}
	details, err := r.store.ModelDetails(req.Context(), modelID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "model not found"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ModelDetailsResponse{
		GeneratedAt:      time.Now().UTC(),
		Model:            details.Model,
		ProviderMetadata: details.ProviderMetadata,
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
