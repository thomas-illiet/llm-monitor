package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/mcpserver"
	"llmservicemonitor/internal/store"
)

// DashboardStore describes the read operations used by HTTP handlers.
type DashboardStore interface {
	ListModelStates(ctx context.Context) ([]store.ModelState, error)
	LatestAuthCheck(ctx context.Context) (*store.CheckRecord, error)
	LatestHTTPCheck(ctx context.Context) (*store.CheckRecord, error)
	KPISummary(ctx context.Context, since time.Time, slo store.SLOThresholds) (store.KPISummary, error)
	KPISummaryForModel(ctx context.Context, modelID string, since time.Time, slo store.SLOThresholds) (store.KPISummary, error)
	RecentModelEvents(ctx context.Context, limit int) ([]store.RecentEvent, error)
	RecentRuns(ctx context.Context, limit int) ([]store.RecentRun, error)
	RecentRunsForModel(ctx context.Context, modelID string, since time.Time, limit int) ([]store.RecentRun, error)
	LatestRunsByModel(ctx context.Context) ([]store.LatestRun, error)
	RecentAlerts(ctx context.Context, limit int) ([]store.RecentAlert, error)
	MetricSamples(ctx context.Context, metric, groupBy string, since time.Time) ([]store.MetricSample, error)
	MetricSamplesForModel(ctx context.Context, metric, groupBy string, since time.Time, modelID string) ([]store.MetricSample, error)
	ModelStatusSamples(ctx context.Context, since time.Time) ([]store.MetricSample, error)
	ListModelEvents(ctx context.Context, query store.ModelEventQuery) (store.ModelEventPage, error)
	ModelPerformance(ctx context.Context, query store.ModelPerformanceQuery) ([]store.ModelPerformanceRow, error)
}

// Router owns API configuration, persistence access, and embedded frontend assets.
type Router struct {
	cfg    config.Config
	store  DashboardStore
	static fs.FS
	logger *slog.Logger
}

// NewRouter registers API endpoints and the embedded frontend fallback handler.
func NewRouter(cfg config.Config, db DashboardStore, static fs.FS, logger *slog.Logger) (http.Handler, error) {
	router := &Router{cfg: cfg, store: db, static: static, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", router.healthz)
	mux.Handle("GET /metrics", router.metricsHandler())
	mux.HandleFunc("GET /api/status", router.status)
	mux.HandleFunc("GET /api/dashboard", router.dashboard)
	mux.HandleFunc("GET /api/model-dashboard", router.modelDashboard)
	mux.HandleFunc("GET /api/model-events", router.modelEvents)
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
