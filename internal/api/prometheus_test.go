package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
)

func TestMetricsEndpointExposesPrometheusText(t *testing.T) {
	now := time.Date(2026, 5, 18, 10, 30, 0, 0, time.UTC)
	missingSince := now.Add(-2 * time.Hour)
	ttft := 120.0
	itl := 40.0
	tpot := 55.0
	outputTPS := 18.5
	inputTokens := 42
	outputTokens := 9
	totalTokens := 51
	vectorDimensions := 1536
	fake := &metricsFakeStore{
		models: []store.ModelState{
			{ModelID: "chat-a", Capability: "chat", Status: "active", LastSeenAt: now},
			{ModelID: "embed-a", Capability: "embedding", Status: "inactive", LastSeenAt: now.Add(-3 * time.Hour), MissingSince: &missingSince},
			{ModelID: "skip-a", Capability: "skip", Status: "active", Excluded: true, LastSeenAt: now},
		},
		httpCheck: &store.CheckRecord{At: now, OK: true, StatusCode: http.StatusOK, LatencyMS: 150},
		authCheck: &store.CheckRecord{At: now.Add(-time.Minute), OK: false, StatusCode: http.StatusServiceUnavailable, LatencyMS: 250},
		latestRuns: []store.LatestRun{
			{
				Capability:            "chat",
				ModelID:               "chat-a",
				StartedAt:             now,
				OK:                    true,
				StatusCode:            http.StatusOK,
				LatencyMS:             900,
				TTFTMS:                &ttft,
				ITLMS:                 &itl,
				TPOTMS:                &tpot,
				InputTokens:           &inputTokens,
				OutputTokens:          &outputTokens,
				TotalTokens:           &totalTokens,
				OutputTokensPerSecond: &outputTPS,
			},
			{
				Capability:       "embedding",
				ModelID:          "embed-a",
				StartedAt:        now.Add(-time.Hour),
				OK:               false,
				StatusCode:       http.StatusTooManyRequests,
				LatencyMS:        1200,
				InputTokens:      &inputTokens,
				TotalTokens:      &inputTokens,
				VectorDimensions: &vectorDimensions,
			},
		},
	}
	handler, err := NewRouter(config.Config{}, fake, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	assertContains(t, body, "# HELP llm_monitor_http_up")
	assertContains(t, body, "llm_monitor_http_up 1")
	assertContains(t, body, "llm_monitor_auth_up 0")
	assertContains(t, body, `llm_monitor_models_total{status="active"} 2`)
	assertContains(t, body, `llm_monitor_models_total{status="inactive"} 1`)
	assertContains(t, body, "llm_monitor_models_skipped_total 1")
	assertContains(t, body, "llm_monitor_model_info")
	assertContains(t, body, `model="chat-a"`)
	assertContains(t, body, "llm_monitor_model_available")
	assertContains(t, body, "llm_monitor_model_probe_success")
	assertContains(t, body, `capability="chat"`)
	assertContains(t, body, `capability="embedding"`)
	assertContains(t, body, "llm_monitor_model_probe_ttft_seconds")
	assertContains(t, body, "llm_monitor_model_probe_vector_dimensions")
	assertContains(t, body, "go_goroutines")
	assertContains(t, body, "process_start_time_seconds")
	assertNotContains(t, body, "provider=")
	assertNotContains(t, body, "error=")
	assertNotContains(t, body, "kind=")
}

func TestMetricsEndpointReportsDownWhenChecksAreMissing(t *testing.T) {
	fake := &metricsFakeStore{}
	handler, err := NewRouter(config.Config{}, fake, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	assertContains(t, body, "llm_monitor_http_up 0")
	assertContains(t, body, "llm_monitor_auth_up 0")
}

type metricsFakeStore struct {
	models     []store.ModelState
	httpCheck  *store.CheckRecord
	authCheck  *store.CheckRecord
	latestRuns []store.LatestRun
}

func (f *metricsFakeStore) ListModelStates(context.Context) ([]store.ModelState, error) {
	return f.models, nil
}

func (f *metricsFakeStore) LatestAuthCheck(context.Context) (*store.CheckRecord, error) {
	return f.authCheck, nil
}

func (f *metricsFakeStore) LatestHTTPCheck(context.Context) (*store.CheckRecord, error) {
	return f.httpCheck, nil
}

func (f *metricsFakeStore) LatestRunsByModel(context.Context) ([]store.LatestRun, error) {
	return f.latestRuns, nil
}

func (f *metricsFakeStore) KPISummary(context.Context, time.Time, store.SLOThresholds) (store.KPISummary, error) {
	return store.KPISummary{}, nil
}

func (f *metricsFakeStore) KPISummaryForModel(context.Context, string, time.Time, store.SLOThresholds) (store.KPISummary, error) {
	return store.KPISummary{}, nil
}

func (f *metricsFakeStore) RecentModelEvents(context.Context, int) ([]store.RecentEvent, error) {
	return nil, nil
}

func (f *metricsFakeStore) RecentRuns(context.Context, int) ([]store.RecentRun, error) {
	return nil, nil
}

func (f *metricsFakeStore) RecentRunsForModel(context.Context, string, time.Time, int) ([]store.RecentRun, error) {
	return nil, nil
}

func (f *metricsFakeStore) RecentAlerts(context.Context, int) ([]store.RecentAlert, error) {
	return nil, nil
}

func (f *metricsFakeStore) MetricSamples(context.Context, string, string, time.Time) ([]store.MetricSample, error) {
	return nil, nil
}

func (f *metricsFakeStore) MetricSamplesForModel(context.Context, string, string, time.Time, string) ([]store.MetricSample, error) {
	return nil, nil
}

func (f *metricsFakeStore) ModelStatusSamples(context.Context, time.Time) ([]store.MetricSample, error) {
	return nil, nil
}

func (f *metricsFakeStore) ListModelEvents(context.Context, store.ModelEventQuery) (store.ModelEventPage, error) {
	return store.ModelEventPage{}, nil
}

func (f *metricsFakeStore) ModelPerformance(context.Context, store.ModelPerformanceQuery) ([]store.ModelPerformanceRow, error) {
	return nil, nil
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("missing %q in:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("found unwanted %q in:\n%s", needle, haystack)
	}
}
