package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"llmservicemonitor/internal/store"
)

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
		OK:          authCheck != nil && authCheck.OK && httpCheck != nil && httpCheck.OK && counts.Inactive == 0,
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
		if status != "" && !modelStatusMatches(model.Status, status) {
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

func modelStatusMatches(modelStatus, filter string) bool {
	if modelStatus == filter {
		return true
	}
	return filter == "missing" && modelStatus == store.ModelStatusInactive
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

func countModels(models []store.ModelState) statusModels {
	counts := statusModels{Total: len(models)}
	for _, model := range models {
		if model.Excluded || model.Capability == "skip" {
			counts.Skipped++
		}
		if model.Status == store.ModelStatusInactive || model.Status == "missing" {
			counts.Inactive++
		}
		if model.Status == store.ModelStatusActive {
			counts.Active++
		}
	}
	counts.Missing = counts.Inactive
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
