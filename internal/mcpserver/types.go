package mcpserver

import (
	"time"

	"llmservicemonitor/internal/store"
)

type emptyInput struct{}

type kpisInput struct {
	Range string `json:"range"`
}

type modelsInput struct {
	Status     string `json:"status"`
	Capability string `json:"capability"`
	Limit      int    `json:"limit"`
}

type modelPerformanceInput struct {
	Range string `json:"range"`
	Sort  string `json:"sort"`
	Limit int    `json:"limit"`
}

type errorOutput struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type statusOutput struct {
	GeneratedAt time.Time    `json:"generated_at"`
	OK          bool         `json:"ok"`
	Models      statusModels `json:"models"`
	Checks      statusChecks `json:"checks"`
}

type statusModels struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Inactive int `json:"inactive"`
	Missing  int `json:"missing"`
	Skipped  int `json:"skipped"`
}

type statusChecks struct {
	Auth checkOutput `json:"auth"`
	HTTP checkOutput `json:"http"`
}

type checkOutput struct {
	OK         bool       `json:"ok"`
	CheckedAt  *time.Time `json:"checked_at"`
	StatusCode int        `json:"status_code"`
	LatencyMS  float64    `json:"latency_ms"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Error      string     `json:"error"`
}

type kpisOutput struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Window      string              `json:"window"`
	Since       time.Time           `json:"since"`
	KPIs        kpisOutputMetrics   `json:"kpis"`
	SLO         store.SLOThresholds `json:"slo"`
}

type kpisOutputMetrics struct {
	TotalRuns             int64   `json:"total_runs"`
	SuccessRate           float64 `json:"success_rate"`
	ErrorCount            int64   `json:"error_count"`
	SLOViolationCount     int64   `json:"slo_violation_count"`
	DegradedModels        int64   `json:"degraded_models"`
	LatencyP50MS          float64 `json:"latency_p50_ms"`
	LatencyP95MS          float64 `json:"latency_p95_ms"`
	LatencyP99MS          float64 `json:"latency_p99_ms"`
	TTFTP99MS             float64 `json:"ttft_p99_ms"`
	ITLP99MS              float64 `json:"itl_p99_ms"`
	OutputTokensPerSecond float64 `json:"output_tokens_per_second"`
	InputTokens           int64   `json:"input_tokens"`
	OutputTokens          int64   `json:"output_tokens"`
}

type modelsOutput struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Filters     modelsOutputFilter `json:"filters"`
	Total       int                `json:"total"`
	Models      []store.ModelState `json:"models"`
}

type modelsOutputFilter struct {
	Status     string `json:"status,omitempty"`
	Capability string `json:"capability,omitempty"`
	Limit      int    `json:"limit"`
}

type modelPerformanceOutput struct {
	GeneratedAt time.Time                   `json:"generated_at"`
	Window      string                      `json:"window"`
	Since       time.Time                   `json:"since"`
	Sort        string                      `json:"sort"`
	Models      []store.ModelPerformanceRow `json:"models"`
}
