package api

import (
	"time"

	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/store"
)

// DashboardResponse is the aggregate payload consumed by the SPA dashboard.
type DashboardResponse struct {
	GeneratedAt        time.Time                `json:"generated_at"`
	Status             StatusResponse           `json:"status"`
	KPIs               store.KPISummary         `json:"kpis"`
	SLO                store.SLOThresholds      `json:"slo"`
	Charts             []ChartResponse          `json:"charts"`
	ModelStatusHistory ChartResponse            `json:"model_status_history"`
	Providers          []ProviderStatusResponse `json:"providers"`
	Models             []store.ModelState       `json:"models"`
	Events             []store.RecentEvent      `json:"events"`
	Runs               []store.RecentRun        `json:"runs"`
	Alerts             []store.RecentAlert      `json:"alerts"`
	Auth               *store.CheckRecord       `json:"auth,omitempty"`
	HTTP               *store.CheckRecord       `json:"http,omitempty"`
	Config             RuntimeConfig            `json:"config"`
}

// ProviderStatusResponse exposes non-secret provider config plus latest checks.
type ProviderStatusResponse struct {
	ID   string             `json:"id"`
	Name string             `json:"name"`
	Auth *store.CheckRecord `json:"auth,omitempty"`
	HTTP *store.CheckRecord `json:"http,omitempty"`
}

// ModelDashboardResponse is the model-scoped payload consumed by the dashboard detail section.
type ModelDashboardResponse struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Model       store.ModelState    `json:"model"`
	KPIs        store.KPISummary    `json:"kpis"`
	SLO         store.SLOThresholds `json:"slo"`
	Charts      []ChartResponse     `json:"charts"`
	Runs        []store.RecentRun   `json:"runs"`
}

// ModelDetailsResponse exposes current model state with provider metadata.
type ModelDetailsResponse struct {
	GeneratedAt      time.Time        `json:"generated_at"`
	Model            store.ModelState `json:"model"`
	ProviderMetadata map[string]any   `json:"provider_metadata"`
}

// StatusResponse is the compact health summary returned by status endpoints.
type StatusResponse struct {
	OK             bool      `json:"ok"`
	GeneratedAt    time.Time `json:"generated_at"`
	ActiveModels   int       `json:"active_models"`
	InactiveModels int       `json:"inactive_models"`
	MissingModels  int       `json:"missing_models"`
	SkippedModels  int       `json:"skipped_models"`
	AuthOK         bool      `json:"auth_ok"`
	HTTPOK         bool      `json:"http_ok"`
}

// RuntimeConfig exposes non-secret runtime settings needed by the SPA.
type RuntimeConfig struct {
	Retention RetentionRuntimeConfig  `json:"retention"`
	SiteName  string                  `json:"site_name"`
	SiteURL   string                  `json:"site_url,omitempty"`
	Providers []ProviderRuntimeConfig `json:"providers"`
}

// ProviderRuntimeConfig exposes non-secret provider labels to the SPA.
type ProviderRuntimeConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RetentionRuntimeConfig exposes the effective retention policy.
type RetentionRuntimeConfig struct {
	HistorySeconds int64 `json:"history_seconds"`
}

// ChartResponse is one chart-ready time series group in the dashboard payload.
type ChartResponse struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Type     string         `json:"type"`
	Metric   string         `json:"metric"`
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
	Error    string         `json:"error,omitempty"`
}

// ChartDataset is one labeled series inside a chart response.
type ChartDataset struct {
	Label string     `json:"label"`
	Data  []*float64 `json:"data"`
}

// ModelEventsResponse is the paginated event timeline returned for one model.
type ModelEventsResponse struct {
	ProviderID string                        `json:"provider_id"`
	ModelID    string                        `json:"model_id"`
	Events     []store.RecentEvent           `json:"events"`
	Total      int64                         `json:"total"`
	Limit      int                           `json:"limit"`
	Offset     int                           `json:"offset"`
	Filters    store.ModelEventFilterOptions `json:"filters"`
}

// RunChecksRequest starts one manual check group from the dashboard.
type RunChecksRequest struct {
	Scope      string `json:"scope"`
	ProviderID string `json:"provider_id,omitempty"`
	ModelID    string `json:"model_id,omitempty"`
}

// RunChecksResponse describes the tasks accepted for manual execution.
type RunChecksResponse struct {
	Jobs []queue.EnqueuedTask `json:"jobs"`
}

// CheckJobsResponse describes the latest queue state for manual jobs.
type CheckJobsResponse struct {
	Jobs []queue.JobStatus `json:"jobs"`
}
