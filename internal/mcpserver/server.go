package mcpserver

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Store describes the read-only persistence methods exposed through MCP tools.
type Store interface {
	ListModelStates(ctx context.Context) ([]store.ModelState, error)
	LatestAuthCheck(ctx context.Context) (*store.CheckRecord, error)
	LatestHTTPCheck(ctx context.Context) (*store.CheckRecord, error)
	KPISummary(ctx context.Context, since time.Time, slo store.SLOThresholds) (store.KPISummary, error)
	ModelPerformance(ctx context.Context, query store.ModelPerformanceQuery) ([]store.ModelPerformanceRow, error)
}

type server struct {
	cfg   config.Config
	store Store
}

// NewHandler builds the protected Streamable HTTP MCP handler.
func NewHandler(cfg config.Config, db Store, logger *slog.Logger) (http.Handler, error) {
	token, err := config.ReadSecret(cfg.MCP.BearerToken, cfg.MCP.BearerTokenFile)
	if err != nil {
		return nil, fmt.Errorf("read mcp bearer token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("mcp bearer token is empty")
	}

	mcpServer := newServer(cfg, db)
	handler := http.Handler(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{
		JSONResponse:   true,
		Logger:         logger,
		SessionTimeout: 30 * time.Minute,
		EventStore:     nil,
		Stateless:      false,
	}))
	handler = bearerAuth(token, handler)
	return handler, nil
}

func newServer(cfg config.Config, db Store) *mcp.Server {
	owner := &server{cfg: cfg, store: db}
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "llm-service-monitor",
		Title:   "LLM Service Monitor",
		Version: "v1.0.0",
	}, nil)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.status",
		Title:       "LLM Monitor Status",
		Description: "Return the current monitor health, model counts, and latest auth/HTTP checks.",
		InputSchema: emptyInputSchema(),
	}, owner.handleStatus)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.kpis",
		Title:       "LLM Monitor KPIs",
		Description: "Return latency, reliability, token, throughput, and SLO KPIs for a recent time window.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"range": map[string]any{
					"type":        "string",
					"description": "Go duration string such as 24h, 168h, or 720h. Defaults to the dashboard window.",
				},
			},
		},
	}, owner.handleKPIs)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.models",
		Title:       "LLM Monitor Models",
		Description: "Return the current model inventory, optionally filtered by status and capability.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"status": map[string]any{
					"type":        "string",
					"description": "Optional model status filter, for example active or missing.",
				},
				"capability": map[string]any{
					"type":        "string",
					"description": "Optional capability filter, for example chat, embedding, skip, or unknown.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     maxLimit,
					"description": "Maximum number of matching models to return.",
				},
			},
		},
	}, owner.handleModels)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.model_performance",
		Title:       "LLM Monitor Model Performance",
		Description: "Return recent run performance aggregated by model.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"range": map[string]any{
					"type":        "string",
					"description": "Go duration string such as 24h, 168h, or 720h. Defaults to the dashboard window.",
				},
				"sort": map[string]any{
					"type":        "string",
					"enum":        []string{"error_count", "success_rate", "avg_latency_ms", "p95_latency_ms", "p99_latency_ms", "model_id"},
					"description": "Sort key. Defaults to error_count.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     maxLimit,
					"description": "Maximum number of model rows to return.",
				},
			},
		},
	}, owner.handleModelPerformance)
	return s
}

func emptyInputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
	}
}

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
	Total   int `json:"total"`
	Active  int `json:"active"`
	Missing int `json:"missing"`
	Skipped int `json:"skipped"`
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

func (s *server) handleStatus(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := decodeArgs[emptyInput](req); err != nil {
		return toolError("invalid_arguments", err.Error())
	}
	models, err := s.store.ListModelStates(ctx)
	if err != nil {
		return toolError("store_error", err.Error())
	}
	authCheck, err := s.store.LatestAuthCheck(ctx)
	if err != nil {
		return toolError("store_error", err.Error())
	}
	httpCheck, err := s.store.LatestHTTPCheck(ctx)
	if err != nil {
		return toolError("store_error", err.Error())
	}

	counts := countModels(models)
	out := statusOutput{
		GeneratedAt: time.Now().UTC(),
		OK:          authCheck != nil && authCheck.OK && httpCheck != nil && httpCheck.OK && counts.Missing == 0,
		Models:      counts,
		Checks: statusChecks{
			Auth: checkToOutput(authCheck),
			HTTP: checkToOutput(httpCheck),
		},
	}
	return toolSuccess(out)
}

func (s *server) handleKPIs(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := decodeArgs[kpisInput](req)
	if err != nil {
		return toolError("invalid_arguments", err.Error())
	}
	window, err := s.parseWindow(input.Range)
	if err != nil {
		return toolError("invalid_range", err.Error())
	}
	now := time.Now().UTC()
	since := now.Add(-window)
	slo := s.sloThresholds()
	kpis, err := s.store.KPISummary(ctx, since, slo)
	if err != nil {
		return toolError("store_error", err.Error())
	}
	out := kpisOutput{
		GeneratedAt: now,
		Window:      window.String(),
		Since:       since,
		KPIs: kpisOutputMetrics{
			TotalRuns:             kpis.TotalRuns,
			SuccessRate:           kpis.SuccessRate,
			ErrorCount:            kpis.ErrorCount,
			SLOViolationCount:     kpis.SLOViolationCount,
			DegradedModels:        kpis.DegradedModels,
			LatencyP50MS:          kpis.LatencyP50MS,
			LatencyP95MS:          kpis.LatencyP95MS,
			LatencyP99MS:          kpis.LatencyP99MS,
			TTFTP99MS:             kpis.TTFTP99MS,
			ITLP99MS:              kpis.ITLP99MS,
			OutputTokensPerSecond: kpis.OutputTokensPerSecond,
			InputTokens:           kpis.InputTokens,
			OutputTokens:          kpis.OutputTokens,
		},
		SLO: slo,
	}
	return toolSuccess(out)
}

func (s *server) handleModels(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := decodeArgs[modelsInput](req)
	if err != nil {
		return toolError("invalid_arguments", err.Error())
	}
	limit := boundedLimit(input.Limit, maxLimit)
	status := strings.TrimSpace(input.Status)
	capability := strings.TrimSpace(input.Capability)

	models, err := s.store.ListModelStates(ctx)
	if err != nil {
		return toolError("store_error", err.Error())
	}
	filtered := make([]store.ModelState, 0, len(models))
	for _, model := range models {
		if status != "" && model.Status != status {
			continue
		}
		if capability != "" && model.Capability != capability {
			continue
		}
		filtered = append(filtered, model)
	}
	total := len(filtered)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	out := modelsOutput{
		GeneratedAt: time.Now().UTC(),
		Filters: modelsOutputFilter{
			Status:     status,
			Capability: capability,
			Limit:      limit,
		},
		Total:  total,
		Models: filtered,
	}
	return toolSuccess(out)
}

func (s *server) handleModelPerformance(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := decodeArgs[modelPerformanceInput](req)
	if err != nil {
		return toolError("invalid_arguments", err.Error())
	}
	window, err := s.parseWindow(input.Range)
	if err != nil {
		return toolError("invalid_range", err.Error())
	}
	sort := strings.TrimSpace(input.Sort)
	if sort == "" {
		sort = "error_count"
	}
	if !validModelPerformanceSort(sort) {
		return toolError("invalid_sort", fmt.Sprintf("unsupported sort %q", sort))
	}
	limit := boundedLimit(input.Limit, maxLimit)
	now := time.Now().UTC()
	since := now.Add(-window)

	rows, err := s.store.ModelPerformance(ctx, store.ModelPerformanceQuery{
		Since: since,
		Sort:  sort,
		Limit: limit,
	})
	if err != nil {
		return toolError("store_error", err.Error())
	}
	out := modelPerformanceOutput{
		GeneratedAt: now,
		Window:      window.String(),
		Since:       since,
		Sort:        sort,
		Models:      rows,
	}
	return toolSuccess(out)
}

func (s *server) parseWindow(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return s.cfg.Dashboard.DefaultWindow.Duration, nil
	}
	window, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid range %q: use Go duration strings such as 24h, 168h, or 720h", raw)
	}
	if window <= 0 {
		return 0, fmt.Errorf("invalid range %q: duration must be positive", raw)
	}
	return window, nil
}

func (s *server) sloThresholds() store.SLOThresholds {
	return store.SLOThresholds{
		TTFTP99MS:           s.cfg.Dashboard.SLO.TTFTP99MS,
		ITLP99MS:            s.cfg.Dashboard.SLO.ITLP99MS,
		RequestLatencyP99MS: s.cfg.Dashboard.SLO.RequestLatencyP99MS,
	}
}

func decodeArgs[T any](req *mcp.CallToolRequest) (T, error) {
	var input T
	raw := json.RawMessage(`{}`)
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		raw = req.Params.Arguments
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return input, errors.New("arguments must contain a single JSON object")
	}
	return input, nil
}

func toolSuccess(output any) (*mcp.CallToolResult, error) {
	return toolResult(output, false)
}

func toolError(code, message string) (*mcp.CallToolResult, error) {
	return toolResult(errorOutput{Error: code, Message: message}, true)
}

func toolResult(output any, isError bool) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		StructuredContent: json.RawMessage(raw),
		IsError:           isError,
	}, nil
}

func countModels(models []store.ModelState) statusModels {
	counts := statusModels{Total: len(models)}
	for _, model := range models {
		if model.Excluded || model.Capability == "skip" {
			counts.Skipped++
		}
		if model.Status == "missing" {
			counts.Missing++
		}
		if model.Status == "active" {
			counts.Active++
		}
	}
	return counts
}

func checkToOutput(check *store.CheckRecord) checkOutput {
	if check == nil {
		return checkOutput{}
	}
	at := check.At
	return checkOutput{
		OK:         check.OK,
		CheckedAt:  &at,
		StatusCode: check.StatusCode,
		LatencyMS:  check.LatencyMS,
		ExpiresAt:  check.ExpiresAt,
		Error:      check.Error,
	}
}

func boundedLimit(value, max int) int {
	if value <= 0 {
		return defaultLimit
	}
	if value > max {
		return max
	}
	return value
}

func validModelPerformanceSort(sort string) bool {
	switch sort {
	case "error_count", "success_rate", "avg_latency_ms", "p95_latency_ms", "p99_latency_ms", "model_id":
		return true
	default:
		return false
	}
}

func bearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		value := strings.TrimSpace(req.Header.Get("Authorization"))
		const prefix = "Bearer "
		if !strings.HasPrefix(value, prefix) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="llm-monitor-mcp"`)
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(strings.TrimPrefix(value, prefix))), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="llm-monitor-mcp"`)
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}
