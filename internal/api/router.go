package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
)

// DashboardStore describes the read operations used by HTTP handlers.
type DashboardStore interface {
	ListModelStates(ctx context.Context) ([]store.ModelState, error)
	LatestAuthCheck(ctx context.Context) (*store.CheckRecord, error)
	LatestHTTPCheck(ctx context.Context) (*store.CheckRecord, error)
	KPISummary(ctx context.Context, since time.Time, slo store.SLOThresholds) (store.KPISummary, error)
	RecentModelEvents(ctx context.Context, limit int) ([]store.RecentEvent, error)
	RecentRuns(ctx context.Context, limit int) ([]store.RecentRun, error)
	RecentAlerts(ctx context.Context, limit int) ([]store.RecentAlert, error)
	MetricSamples(ctx context.Context, metric, groupBy string, since time.Time) ([]store.MetricSample, error)
	ModelStatusSamples(ctx context.Context, since time.Time) ([]store.MetricSample, error)
	ListModelEvents(ctx context.Context, query store.ModelEventQuery) (store.ModelEventPage, error)
}

// Router owns API configuration, persistence access, and embedded frontend assets.
type Router struct {
	cfg    config.Config
	store  DashboardStore
	static fs.FS
	logger *slog.Logger
}

// NewRouter registers API endpoints and the embedded frontend fallback handler.
func NewRouter(cfg config.Config, db DashboardStore, static fs.FS, logger *slog.Logger) http.Handler {
	router := &Router{cfg: cfg, store: db, static: static, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", router.healthz)
	mux.HandleFunc("GET /api/status", router.status)
	mux.HandleFunc("GET /api/dashboard", router.dashboard)
	mux.HandleFunc("GET /api/model-events", router.modelEvents)
	mux.HandleFunc("/", router.frontend)
	return mux
}
