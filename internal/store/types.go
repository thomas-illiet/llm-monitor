package store

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store owns the PostgreSQL connection pool used by repository methods.
type Store struct {
	pool *pgxpool.Pool
}

// ObservedModel is one model returned by the current inventory snapshot.
type ObservedModel struct {
	ID           string         `json:"id"`
	Capability   string         `json:"capability"`
	Excluded     bool           `json:"excluded"`
	SkipReason   string         `json:"skip_reason,omitempty"`
	ProbeDetails map[string]any `json:"probe_details,omitempty"`
}

// ModelEvent is the lifecycle event data returned to alert orchestration.
type ModelEvent struct {
	ID              int64
	ModelID         string
	EventType       string
	Capability      string
	ObservedAt      time.Time
	MissingSince    *time.Time
	MissingDuration time.Duration
	FirstSeen       bool
}

// ModelState is the persisted current state of one model.
type ModelState struct {
	ModelID      string     `json:"model_id"`
	Capability   string     `json:"capability"`
	Excluded     bool       `json:"excluded"`
	Status       string     `json:"status"`
	FirstSeenAt  time.Time  `json:"first_seen_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	MissingSince *time.Time `json:"missing_since,omitempty"`
	SkipReason   string     `json:"skip_reason,omitempty"`
	LastProbeAt  *time.Time `json:"last_probe_at,omitempty"`
}

// CheckRecord stores one auth or target HTTP availability check.
type CheckRecord struct {
	At         time.Time  `json:"at"`
	OK         bool       `json:"ok"`
	StatusCode int        `json:"status_code"`
	LatencyMS  float64    `json:"latency_ms"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Error      string     `json:"error"`
}

// ChatRunRecord stores metrics captured from one chat completion probe.
type ChatRunRecord struct {
	ModelID               string
	PromptID              string
	StartedAt             time.Time
	OK                    bool
	StatusCode            int
	LatencyMS             float64
	TTFTMS                *float64
	ITLMS                 *float64
	TPOTMS                *float64
	RequestLatencyMS      *float64
	InputTokens           *int
	OutputTokens          *int
	TotalTokens           *int
	OutputTokensPerSecond *float64
	Error                 string
}

// EmbeddingRunRecord stores metrics captured from one embedding probe.
type EmbeddingRunRecord struct {
	ModelID          string
	FixturePath      string
	FixtureBytes     int
	StartedAt        time.Time
	OK               bool
	StatusCode       int
	LatencyMS        float64
	InputTokens      *int
	TotalTokens      *int
	VectorDimensions *int
	Error            string
}

// EmailAlertRecord stores one attempted or successful model alert email.
type EmailAlertRecord struct {
	AlertKey string
	ModelID  string
	Type     string
	SentAt   time.Time
	Subject  string
	To       []string
	Error    string
}

// RecentRun is a dashboard timeline row for recent chat and embedding probes.
type RecentRun struct {
	Kind         string    `json:"kind"`
	ModelID      string    `json:"model_id"`
	PromptID     string    `json:"prompt_id,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	OK           bool      `json:"ok"`
	StatusCode   int       `json:"status_code"`
	LatencyMS    float64   `json:"latency_ms"`
	InputTokens  *int      `json:"input_tokens,omitempty"`
	OutputTokens *int      `json:"output_tokens,omitempty"`
	TotalTokens  *int      `json:"total_tokens,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// RecentEvent is a dashboard timeline row for model-scoped diagnostic events.
type RecentEvent struct {
	ID         int64          `json:"id"`
	ModelID    string         `json:"model_id"`
	EventType  string         `json:"event_type"`
	Source     string         `json:"source"`
	Severity   string         `json:"severity"`
	Status     string         `json:"status"`
	Capability string         `json:"capability"`
	ObservedAt time.Time      `json:"observed_at"`
	Title      string         `json:"title"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
}

// ModelEventQuery contains filters and pagination for a model event page.
type ModelEventQuery struct {
	ModelID    string
	Limit      int
	Offset     int
	Statuses   []string
	Sources    []string
	EventTypes []string
}

// ModelEventFilterOptions lists the available event filters for one model.
type ModelEventFilterOptions struct {
	Statuses   []string `json:"statuses"`
	Sources    []string `json:"sources"`
	EventTypes []string `json:"event_types"`
}

// ModelEventPage contains one page of model events plus filter metadata.
type ModelEventPage struct {
	Events  []RecentEvent
	Total   int64
	Filters ModelEventFilterOptions
}

// ModelEventRecord is the write model for one model-scoped diagnostic event.
type ModelEventRecord struct {
	ModelID    string
	EventType  string
	Source     string
	Severity   string
	Status     string
	Capability string
	ObservedAt time.Time
	Title      string
	Message    string
	Details    map[string]any
}

// RecentAlert is a dashboard row for a recent model lifecycle email alert.
type RecentAlert struct {
	ModelID    string    `json:"model_id"`
	Type       string    `json:"type"`
	SentAt     time.Time `json:"sent_at"`
	Subject    string    `json:"subject"`
	Recipients []string  `json:"recipients"`
	Error      string    `json:"error,omitempty"`
}

// KPISummary aggregates recent latency, throughput, reliability, and token metrics.
type KPISummary struct {
	TotalRuns             int64   `json:"total_runs"`
	SuccessRate           float64 `json:"success_rate"`
	LatencyP50MS          float64 `json:"latency_p50_ms"`
	LatencyP95MS          float64 `json:"latency_p95_ms"`
	LatencyP99MS          float64 `json:"latency_p99_ms"`
	RequestLatencyP50MS   float64 `json:"request_latency_p50_ms"`
	RequestLatencyP90MS   float64 `json:"request_latency_p90_ms"`
	RequestLatencyP95MS   float64 `json:"request_latency_p95_ms"`
	RequestLatencyP99MS   float64 `json:"request_latency_p99_ms"`
	TTFTP50MS             float64 `json:"ttft_p50_ms"`
	TTFTP90MS             float64 `json:"ttft_p90_ms"`
	TTFTP95MS             float64 `json:"ttft_p95_ms"`
	TTFTP99MS             float64 `json:"ttft_p99_ms"`
	ITLP50MS              float64 `json:"itl_p50_ms"`
	ITLP90MS              float64 `json:"itl_p90_ms"`
	ITLP95MS              float64 `json:"itl_p95_ms"`
	ITLP99MS              float64 `json:"itl_p99_ms"`
	TPOTP50MS             float64 `json:"tpot_p50_ms"`
	TPOTP90MS             float64 `json:"tpot_p90_ms"`
	TPOTP95MS             float64 `json:"tpot_p95_ms"`
	TPOTP99MS             float64 `json:"tpot_p99_ms"`
	OutputTokensPerSecond float64 `json:"output_tokens_per_second"`
	SLOViolationCount     int64   `json:"slo_violation_count"`
	DegradedModels        int64   `json:"degraded_models"`
	ErrorCount            int64   `json:"error_count"`
	InputTokens           int64   `json:"input_tokens"`
	OutputTokens          int64   `json:"output_tokens"`
}

// SLOThresholds contains the dashboard thresholds used to classify degradation.
type SLOThresholds struct {
	TTFTP99MS           float64 `json:"ttft_p99_ms"`
	ITLP99MS            float64 `json:"itl_p99_ms"`
	RequestLatencyP99MS float64 `json:"request_latency_p99_ms"`
}

// MetricSample is a raw time-series point before API chart bucketing.
type MetricSample struct {
	At         time.Time
	ModelID    string
	Capability string
	Group      string
	Value      float64
}

// ModelPerformanceQuery controls the read-only MCP model performance summary.
type ModelPerformanceQuery struct {
	Since time.Time
	Sort  string
	Limit int
}

// ModelPerformanceError describes the latest failed run for one model.
type ModelPerformanceError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

// ModelPerformanceRow aggregates recent probe performance for one model.
type ModelPerformanceRow struct {
	ModelID      string                 `json:"model_id"`
	Runs         int64                  `json:"runs"`
	SuccessRate  float64                `json:"success_rate"`
	ErrorCount   int64                  `json:"error_count"`
	AvgLatencyMS float64                `json:"avg_latency_ms"`
	P50LatencyMS float64                `json:"p50_latency_ms"`
	P95LatencyMS float64                `json:"p95_latency_ms"`
	P99LatencyMS float64                `json:"p99_latency_ms"`
	FirstRunAt   time.Time              `json:"first_run_at"`
	LastRunAt    time.Time              `json:"last_run_at"`
	LatestError  *ModelPerformanceError `json:"latest_error,omitempty"`
}
