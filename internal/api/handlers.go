package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/store"

	"github.com/jackc/pgx/v5"
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

// runChecks enqueues manual health, inventory, or model probe work from the dashboard.
func (r *Router) runChecks(w http.ResponseWriter, req *http.Request) {
	if r.taskQueue == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "task queue is not configured"})
		return
	}
	var payload RunChecksRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	payload.Scope = strings.TrimSpace(strings.ToLower(payload.Scope))
	payload.ModelID = strings.TrimSpace(payload.ModelID)
	var jobs []queue.EnqueuedTask
	var err error
	switch payload.Scope {
	case "all":
		jobs, err = r.enqueueAllChecks(req)
	case "model":
		if payload.ModelID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_id is required for model scope"})
			return
		}
		jobs, err = r.enqueueModelCheck(req, payload.ModelID)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be all or model"})
		return
	}
	if err != nil {
		switch err.(type) {
		case errModelNotRunnable:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		case errRunnableModelsUnavailable:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, RunChecksResponse{Jobs: jobs})
}

func (r *Router) enqueueAllChecks(req *http.Request) ([]queue.EnqueuedTask, error) {
	ctx := req.Context()
	jobs := make([]queue.EnqueuedTask, 0)
	httpJob, err := r.taskQueue.EnqueueHTTPCheck(ctx)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, httpJob)
	authJob, err := r.taskQueue.EnqueueAuthCheck(ctx)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, authJob)
	snapshotJob, err := r.taskQueue.EnqueueModelSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, snapshotJob)
	modelStore, ok := r.store.(runnableModelStore)
	if !ok {
		return jobs, nil
	}
	models, err := modelStore.RunnableModels(ctx)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		job, err := r.taskQueue.EnqueueModelRun(ctx, model, "manual")
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *Router) enqueueModelCheck(req *http.Request, modelID string) ([]queue.EnqueuedTask, error) {
	modelStore, ok := r.store.(runnableModelStore)
	if !ok {
		return nil, errRunnableModelsUnavailable{}
	}
	models, err := modelStore.RunnableModels(req.Context())
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		if model.ModelID == modelID {
			job, err := r.taskQueue.EnqueueModelRun(req.Context(), model, "manual")
			if err != nil {
				return nil, err
			}
			return []queue.EnqueuedTask{job}, nil
		}
	}
	return nil, errModelNotRunnable{modelID: modelID}
}

// checkJobs returns manual queue status used by dashboard spinners.
func (r *Router) checkJobs(w http.ResponseWriter, req *http.Request) {
	if r.taskQueue == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "task queue is not configured"})
		return
	}
	ids := parseJobIDs(req.URL.Query()["ids"])
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids query parameter is required"})
		return
	}
	statuses, err := r.taskQueue.InspectJobs(req.Context(), ids)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, CheckJobsResponse{Jobs: statuses})
}

type errRunnableModelsUnavailable struct{}

func (errRunnableModelsUnavailable) Error() string {
	return "runnable model store is not available"
}

type errModelNotRunnable struct {
	modelID string
}

func (e errModelNotRunnable) Error() string {
	return "model " + e.modelID + " is not runnable"
}

func parseJobIDs(values []string) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
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
