package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/mcpserver"
	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/store"
)

// DashboardStore describes the read operations used by HTTP handlers.
type DashboardStore interface {
	ListModelStates(ctx context.Context) ([]store.ModelState, error)
	LatestAuthCheck(ctx context.Context, providerID string) (*store.CheckRecord, error)
	LatestHTTPCheck(ctx context.Context, providerID string) (*store.CheckRecord, error)
	KPISummary(ctx context.Context, since time.Time, slo store.SLOThresholds) (store.KPISummary, error)
	KPISummaryForModel(ctx context.Context, providerID, modelID string, since time.Time, slo store.SLOThresholds) (store.KPISummary, error)
	RecentModelEvents(ctx context.Context, limit int) ([]store.RecentEvent, error)
	RecentRuns(ctx context.Context, limit int) ([]store.RecentRun, error)
	RecentRunsForModel(ctx context.Context, providerID, modelID string, since time.Time, limit int) ([]store.RecentRun, error)
	LatestRunsByModel(ctx context.Context) ([]store.LatestRun, error)
	RecentAlerts(ctx context.Context, limit int) ([]store.RecentAlert, error)
	ModelDetails(ctx context.Context, providerID, modelID string) (*store.ModelDetails, error)
	MetricSamples(ctx context.Context, metric, groupBy string, since time.Time) ([]store.MetricSample, error)
	MetricSamplesForModel(ctx context.Context, metric, groupBy string, since time.Time, providerID, modelID string) ([]store.MetricSample, error)
	ModelStatusSamples(ctx context.Context, since time.Time) ([]store.MetricSample, error)
	ListModelEvents(ctx context.Context, query store.ModelEventQuery) (store.ModelEventPage, error)
	ModelPerformance(ctx context.Context, query store.ModelPerformanceQuery) ([]store.ModelPerformanceRow, error)
}

// ManualTaskQueue describes the queue operations used by manual dashboard checks.
type ManualTaskQueue interface {
	EnqueueHTTPCheck(ctx context.Context, providerID string) (queue.EnqueuedTask, error)
	EnqueueAuthCheck(ctx context.Context, providerID string) (queue.EnqueuedTask, error)
	EnqueueModelSnapshot(ctx context.Context, providerID string) (queue.EnqueuedTask, error)
	EnqueueModelRun(ctx context.Context, model store.RunnableModel, reason string) (queue.EnqueuedTask, error)
	InspectJobs(ctx context.Context, ids []string) ([]queue.JobStatus, error)
}

type modelRunScheduleReader interface {
	ScheduledModelRuns(ctx context.Context) (map[string]time.Time, error)
}

type runnableModelStore interface {
	RunnableModels(ctx context.Context) ([]store.RunnableModel, error)
}

// Router owns API configuration, persistence access, and embedded frontend assets.
type Router struct {
	cfg       config.Config
	store     DashboardStore
	static    fs.FS
	logger    *slog.Logger
	taskQueue ManualTaskQueue
}

// NewRouter registers API endpoints and the embedded frontend fallback handler.
func NewRouter(cfg config.Config, db DashboardStore, static fs.FS, logger *slog.Logger, queues ...ManualTaskQueue) (http.Handler, error) {
	var taskQueue ManualTaskQueue
	if len(queues) > 0 {
		taskQueue = queues[0]
	}
	router := &Router{cfg: cfg, store: db, static: static, logger: logger, taskQueue: taskQueue}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", router.healthz)
	mux.Handle("GET /metrics", router.metricsHandler())
	mux.HandleFunc("GET /api/status", router.status)
	mux.HandleFunc("GET /api/dashboard", router.dashboard)
	mux.HandleFunc("GET /api/providers", router.providers)
	mux.HandleFunc("GET /api/providers/{provider_id}/dashboard", router.modelDashboard)
	mux.HandleFunc("GET /api/providers/{provider_id}/models/{model_key}/events", router.modelEvents)
	mux.HandleFunc("GET /api/providers/{provider_id}/models/{model_key}/details", router.modelDetails)
	mux.HandleFunc("POST /api/checks/runs", router.runChecks)
	mux.HandleFunc("GET /api/checks/jobs", router.checkJobs)
	if cfg.MCP.Enabled {
		handler, err := mcpserver.NewHandler(cfg, db, logger)
		if err != nil {
			return nil, err
		}
		mux.Handle(cfg.MCP.Path, handler)
	}
	mux.HandleFunc("/", router.frontend)
	return mux, nil
}
