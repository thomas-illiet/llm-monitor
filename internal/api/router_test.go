package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
)

// TestModelStatusSamplesFromStates verifies status samples include active, inactive, and skipped counts.
func TestModelStatusSamplesFromStates(t *testing.T) {
	at := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	samples := modelStatusSamplesFromStates([]store.ModelState{
		{ModelID: "chat", Capability: "chat", Status: "active"},
		{ModelID: "embedding", Capability: "embedding", Status: "inactive"},
		{ModelID: "skipped-active", Capability: "skip", Status: "active"},
		{ModelID: "skipped-inactive", Capability: "chat", Excluded: true, Status: "inactive"},
	}, at)

	got := map[string]float64{}
	for _, sample := range samples {
		if !sample.At.Equal(at) {
			t.Fatalf("sample time = %s, want %s", sample.At, at)
		}
		got[sample.Group] = sample.Value
	}

	want := map[string]float64{
		"active":   2,
		"inactive": 2,
		"skipped":  2,
	}
	for group, value := range want {
		if got[group] != value {
			t.Fatalf("sample %q = %v, want %v", group, got[group], value)
		}
	}
}

// TestModelStatusInterval verifies chart windows choose the intended bucket interval.
func TestModelStatusInterval(t *testing.T) {
	tests := []struct {
		name   string
		window time.Duration
		want   time.Duration
	}{
		{name: "day", window: 24 * time.Hour, want: time.Hour},
		{name: "week", window: 7 * 24 * time.Hour, want: 6 * time.Hour},
		{name: "month", window: 30 * 24 * time.Hour, want: 24 * time.Hour},
		{name: "year", window: 365 * 24 * time.Hour, want: 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modelStatusInterval(tt.window); got != tt.want {
				t.Fatalf("modelStatusInterval(%s) = %s, want %s", tt.window, got, tt.want)
			}
		})
	}
}

// TestStaticDashboardCharts verifies the dashboard chart set is code-owned.
func TestStaticDashboardCharts(t *testing.T) {
	got := make([]string, 0, len(dashboardCharts))
	gotTypes := map[string]string{}
	for _, chart := range dashboardCharts {
		got = append(got, chart.ID)
		gotTypes[chart.ID] = chart.Type
	}
	assertStrings(t, got, []string{
		"ttft-by-model",
		"request-latency-by-model",
		"http-latency",
	})
	assertChartTypes(t, gotTypes, map[string]string{
		"ttft-by-model":            "line",
		"request-latency-by-model": "line",
		"http-latency":             "line",
	})
}

// TestStaticModelDashboardCharts verifies model detail chart sets are capability-aware.
func TestStaticModelDashboardCharts(t *testing.T) {
	got := make([]string, 0, len(modelDashboardChartConfigs("chat")))
	gotTypes := map[string]string{}
	for _, chart := range modelDashboardChartConfigs("chat") {
		got = append(got, chart.ID)
		gotTypes[chart.ID] = chart.Type
	}
	assertStrings(t, got, []string{
		"model-request-latency",
		"model-ttft",
		"model-itl",
		"model-tpot",
		"model-output-throughput",
		"model-errors",
	})
	assertChartTypes(t, gotTypes, map[string]string{
		"model-request-latency":   "bar",
		"model-ttft":              "bar",
		"model-itl":               "bar",
		"model-tpot":              "bar",
		"model-output-throughput": "bar",
		"model-errors":            "stacked-bar",
	})

	got = make([]string, 0, len(modelDashboardChartConfigs("embedding")))
	gotTypes = map[string]string{}
	for _, chart := range modelDashboardChartConfigs("embedding") {
		got = append(got, chart.ID)
		gotTypes[chart.ID] = chart.Type
	}
	assertStrings(t, got, []string{
		"model-request-latency",
		"model-input-tokens",
		"model-vector-dimensions",
		"model-errors",
	})
	assertChartTypes(t, gotTypes, map[string]string{
		"model-request-latency":   "bar",
		"model-input-tokens":      "bar",
		"model-vector-dimensions": "bar",
		"model-errors":            "stacked-bar",
	})
}

// TestDashboardChartInterval verifies static charts adapt to the active KPI window.
func TestDashboardChartInterval(t *testing.T) {
	tests := []struct {
		name   string
		window time.Duration
		want   time.Duration
	}{
		{name: "day", window: 24 * time.Hour, want: 30 * time.Minute},
		{name: "week", window: 7 * 24 * time.Hour, want: 6 * time.Hour},
		{name: "month", window: 30 * 24 * time.Hour, want: 24 * time.Hour},
		{name: "year", window: 365 * 24 * time.Hour, want: 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashboardChartInterval(tt.window); got != tt.want {
				t.Fatalf("dashboardChartInterval(%s) = %s, want %s", tt.window, got, tt.want)
			}
		})
	}
}

// TestFormatBucketLabel verifies timeline labels stay readable across KPI ranges.
func TestFormatBucketLabel(t *testing.T) {
	at := time.Date(2026, 5, 16, 12, 30, 0, 0, time.UTC)
	tests := []struct {
		name   string
		window time.Duration
		want   string
	}{
		{name: "day", window: 24 * time.Hour, want: "12:30"},
		{name: "week", window: 7 * 24 * time.Hour, want: "Sat 12:30"},
		{name: "month", window: 30 * 24 * time.Hour, want: "May 16"},
		{name: "year", window: 365 * 24 * time.Hour, want: "May 16"},
		{name: "multi-year", window: 730 * 24 * time.Hour, want: "May 2026"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBucketLabel(at, tt.window); got != tt.want {
				t.Fatalf("formatBucketLabel(%s) = %q, want %q", tt.window, got, tt.want)
			}
		})
	}
}

// TestBucketSamplesAveragesAndFiltersModels verifies metric bucketing respects model filters.
func TestBucketSamplesAveragesAndFiltersModels(t *testing.T) {
	since := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	labels, datasets := bucketSamples([]store.MetricSample{
		{At: since.Add(10 * time.Minute), ModelID: "model-a", Group: "chat", Value: 100},
		{At: since.Add(20 * time.Minute), ModelID: "model-a", Group: "chat", Value: 200},
		{At: since.Add(20 * time.Minute), ModelID: "model-b", Group: "chat", Value: 900},
		{At: since.Add(70 * time.Minute), ModelID: "model-a", Group: "chat", Value: 300},
	}, []string{"model-a"}, since, since.Add(2*time.Hour), time.Hour, false)

	if len(labels) != 3 {
		t.Fatalf("labels len = %d, want 3", len(labels))
	}
	if len(datasets) != 1 {
		t.Fatalf("datasets len = %d, want 1", len(datasets))
	}
	got := datasets[0].Data
	assertChartData(t, got, []*float64{chartValue(150), chartValue(300), nil})
}

// TestBucketSamplesSumsEmptyBucketsAsZero verifies summed metrics keep a zero baseline.
func TestBucketSamplesSumsEmptyBucketsAsZero(t *testing.T) {
	since := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	_, datasets := bucketSamples([]store.MetricSample{
		{At: since.Add(10 * time.Minute), ModelID: "model-a", Group: "chat", Value: 1},
		{At: since.Add(20 * time.Minute), ModelID: "model-a", Group: "chat", Value: 1},
	}, nil, since, since.Add(2*time.Hour), time.Hour, true)

	if len(datasets) != 1 {
		t.Fatalf("datasets len = %d, want 1", len(datasets))
	}
	assertChartData(t, datasets[0].Data, []*float64{chartValue(2), chartValue(0), chartValue(0)})
}

// TestHTTPCheckLatencyChartSkipsAuthWhenDisabled verifies static auth checks do not appear in the HTTP latency chart.
func TestHTTPCheckLatencyChartSkipsAuthWhenDisabled(t *testing.T) {
	since := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	fake := &chartMetricStore{samples: map[string][]store.MetricSample{
		"http_latency_ms|check": {
			{At: since.Add(10 * time.Minute), Group: "http", Value: 120},
		},
		"auth_latency_ms|check": {
			{At: since.Add(10 * time.Minute), Group: "auth", Value: 45},
		},
	}}
	router := Router{store: fake}

	chart := router.buildChart(context.Background(), dashboardChartConfig{
		ID:      "http-latency",
		Title:   "HTTP check latency",
		Type:    "line",
		Metric:  "http_latency_ms",
		GroupBy: "check",
	}, since, since.Add(time.Hour), time.Hour)

	if chart.Error != "" {
		t.Fatalf("chart error = %q, want none", chart.Error)
	}
	assertStrings(t, fake.requests, []string{"http_latency_ms|check"})
	if len(chart.Datasets) != 1 {
		t.Fatalf("datasets len = %d, want 1", len(chart.Datasets))
	}
	if chart.Datasets[0].Label != targetHTTPLatencyLabel {
		t.Fatalf("dataset label = %q, want %q", chart.Datasets[0].Label, targetHTTPLatencyLabel)
	}
	assertChartData(t, chart.Datasets[0].Data, []*float64{chartValue(120), nil, nil})
}

// TestHTTPCheckLatencyChartAddsAuthWhenEnabled verifies OAuth provider latency shares the HTTP latency card.
func TestHTTPCheckLatencyChartAddsAuthWhenEnabled(t *testing.T) {
	since := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	fake := &chartMetricStore{samples: map[string][]store.MetricSample{
		"http_latency_ms|check": {
			{At: since.Add(10 * time.Minute), Group: "http", Value: 120},
		},
		"auth_latency_ms|check": {
			{At: since.Add(40 * time.Minute), Group: "auth", Value: 45},
		},
	}}
	router := Router{
		cfg:   config.Config{Auth: config.AuthConfig{Enabled: true}},
		store: fake,
	}

	chart := router.buildChart(context.Background(), dashboardChartConfig{
		ID:      "http-latency",
		Title:   "HTTP check latency",
		Type:    "line",
		Metric:  "http_latency_ms",
		GroupBy: "check",
	}, since, since.Add(time.Hour), time.Hour)

	if chart.Error != "" {
		t.Fatalf("chart error = %q, want none", chart.Error)
	}
	assertStrings(t, fake.requests, []string{"http_latency_ms|check", "auth_latency_ms|check"})
	if len(chart.Datasets) != 2 {
		t.Fatalf("datasets len = %d, want 2", len(chart.Datasets))
	}
	if chart.Datasets[0].Label != targetHTTPLatencyLabel || chart.Datasets[1].Label != authProviderLatencyLabel {
		t.Fatalf("dataset labels = %q, %q; want %q, %q", chart.Datasets[0].Label, chart.Datasets[1].Label, targetHTTPLatencyLabel, authProviderLatencyLabel)
	}
	assertChartData(t, chart.Datasets[0].Data, []*float64{chartValue(120), nil, nil})
	assertChartData(t, chart.Datasets[1].Data, []*float64{nil, chartValue(45), nil})
}

// TestHTTPCheckLatencyChartReportsAuthQueryError verifies auth metric failures surface on the chart.
func TestHTTPCheckLatencyChartReportsAuthQueryError(t *testing.T) {
	since := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	fake := &chartMetricStore{
		samples: map[string][]store.MetricSample{
			"http_latency_ms|check": {
				{At: since.Add(10 * time.Minute), Group: "http", Value: 120},
			},
		},
		errors: map[string]error{
			"auth_latency_ms|check": errors.New("auth query failed"),
		},
	}
	router := Router{
		cfg:   config.Config{Auth: config.AuthConfig{Enabled: true}},
		store: fake,
	}

	chart := router.buildChart(context.Background(), dashboardChartConfig{
		ID:      "http-latency",
		Title:   "HTTP check latency",
		Type:    "line",
		Metric:  "http_latency_ms",
		GroupBy: "check",
	}, since, since.Add(time.Hour), time.Hour)

	if chart.Error != "auth query failed" {
		t.Fatalf("chart error = %q, want auth query failed", chart.Error)
	}
	assertStrings(t, fake.requests, []string{"http_latency_ms|check", "auth_latency_ms|check"})
}

// TestParseModelEventsQueryDefaultsAndFilters verifies model event query normalization.
func TestParseModelEventsQueryDefaultsAndFilters(t *testing.T) {
	values := url.Values{}
	values.Set("model_id", " test-model ")
	values.Add("status", "ok")
	values.Add("status", "")
	values.Add("status", "error")
	values.Add("status", "ok")
	values.Add("source", " scheduler ")
	values.Add("source", "monitor")
	values.Add("event_type", "scheduled_run")

	got, errMessage := parseModelEventsQuery(values)
	if errMessage != "" {
		t.Fatalf("parseModelEventsQuery returned error %q", errMessage)
	}
	if got.ModelID != "test-model" {
		t.Fatalf("ModelID = %q, want test-model", got.ModelID)
	}
	if got.Limit != 25 {
		t.Fatalf("Limit = %d, want 25", got.Limit)
	}
	if got.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", got.Offset)
	}
	assertStrings(t, got.Statuses, []string{"error", "ok"})
	assertStrings(t, got.Sources, []string{"monitor", "scheduler"})
	assertStrings(t, got.EventTypes, []string{"scheduled_run"})
}

// TestParseModelEventsQueryBounds verifies pagination values are capped or defaulted.
func TestParseModelEventsQueryBounds(t *testing.T) {
	values := url.Values{}
	values.Set("model_id", "test-model")
	values.Set("limit", "500")
	values.Set("offset", "-20")

	got, errMessage := parseModelEventsQuery(values)
	if errMessage != "" {
		t.Fatalf("parseModelEventsQuery returned error %q", errMessage)
	}
	if got.Limit != 100 {
		t.Fatalf("Limit = %d, want capped 100", got.Limit)
	}
	if got.Offset != 0 {
		t.Fatalf("Offset = %d, want fallback 0", got.Offset)
	}

	values.Set("limit", "not-a-number")
	values.Set("offset", "also-bad")
	got, errMessage = parseModelEventsQuery(values)
	if errMessage != "" {
		t.Fatalf("parseModelEventsQuery returned error %q", errMessage)
	}
	if got.Limit != 25 {
		t.Fatalf("Limit = %d, want fallback 25", got.Limit)
	}
	if got.Offset != 0 {
		t.Fatalf("Offset = %d, want fallback 0", got.Offset)
	}
}

// TestParseModelEventsQueryRequiresModelID verifies model event queries are model-scoped.
func TestParseModelEventsQueryRequiresModelID(t *testing.T) {
	_, errMessage := parseModelEventsQuery(url.Values{})
	if errMessage != "model_id is required" {
		t.Fatalf("error = %q, want model_id is required", errMessage)
	}
}

// TestParseModelDashboardQuery verifies model dashboard query normalization.
func TestParseModelDashboardQuery(t *testing.T) {
	values := url.Values{}
	values.Set("model_id", " test-model ")
	values.Set("range", "12h")

	got, errMessage := parseModelDashboardQuery(values, 24*time.Hour)
	if errMessage != "" {
		t.Fatalf("parseModelDashboardQuery returned error %q", errMessage)
	}
	if got.ModelID != "test-model" {
		t.Fatalf("ModelID = %q, want test-model", got.ModelID)
	}
	if got.Window != 12*time.Hour {
		t.Fatalf("Window = %s, want 12h", got.Window)
	}
}

// TestParseModelDashboardQueryFallbacks verifies missing model and invalid ranges are handled.
func TestParseModelDashboardQueryFallbacks(t *testing.T) {
	_, errMessage := parseModelDashboardQuery(url.Values{}, 24*time.Hour)
	if errMessage != "model_id is required" {
		t.Fatalf("error = %q, want model_id is required", errMessage)
	}

	values := url.Values{}
	values.Set("model_id", "test-model")
	values.Set("range", "not-a-duration")
	got, errMessage := parseModelDashboardQuery(values, 24*time.Hour)
	if errMessage != "" {
		t.Fatalf("parseModelDashboardQuery returned error %q", errMessage)
	}
	if got.Window != 24*time.Hour {
		t.Fatalf("Window = %s, want fallback 24h", got.Window)
	}

	values.Set("range", "-1h")
	got, errMessage = parseModelDashboardQuery(values, 24*time.Hour)
	if errMessage != "" {
		t.Fatalf("parseModelDashboardQuery returned error %q", errMessage)
	}
	if got.Window != 24*time.Hour {
		t.Fatalf("Window = %s, want fallback 24h for negative range", got.Window)
	}

	values.Set("range", "0s")
	got, errMessage = parseModelDashboardQuery(values, 24*time.Hour)
	if errMessage != "" {
		t.Fatalf("parseModelDashboardQuery returned error %q", errMessage)
	}
	if got.Window != 24*time.Hour {
		t.Fatalf("Window = %s, want fallback 24h for zero range", got.Window)
	}
}

// TestParseDashboardWindowRejectsNonPositiveRanges verifies global dashboard ranges cannot invert windows.
func TestParseDashboardWindowRejectsNonPositiveRanges(t *testing.T) {
	for _, raw := range []string{"-1h", "0s"} {
		values := url.Values{}
		values.Set("range", raw)
		if got := parseDashboardWindow(values, 24*time.Hour); got != 24*time.Hour {
			t.Fatalf("parseDashboardWindow(%q) = %s, want fallback 24h", raw, got)
		}
	}
}

// TestCapDashboardWindowAppliesRetention verifies dashboard ranges cannot exceed retention.
func TestCapDashboardWindowAppliesRetention(t *testing.T) {
	if got := capDashboardWindow(365*24*time.Hour, 90*24*time.Hour); got != 90*24*time.Hour {
		t.Fatalf("capDashboardWindow() = %s, want 2160h", got)
	}
	if got := capDashboardWindow(30*24*time.Hour, 90*24*time.Hour); got != 30*24*time.Hour {
		t.Fatalf("capDashboardWindow() = %s, want 720h", got)
	}
	if got := capDashboardWindow(365*24*time.Hour, 0); got != 365*24*time.Hour {
		t.Fatalf("capDashboardWindow() = %s, want uncapped 8760h", got)
	}
}

// TestRuntimeConfigExposesRetentionHistory verifies non-secret retention config is serialized for the SPA.
func TestRuntimeConfigExposesRetentionHistory(t *testing.T) {
	router := Router{cfg: config.Config{
		Dashboard: config.DashboardConfig{
			SiteName: "Platform Monitor",
			SiteURL:  "https://monitor.example.test",
		},
		Retention: config.RetentionConfig{History: config.Duration{Duration: 90 * 24 * time.Hour}},
	}}

	got := router.runtimeConfig()

	if got.Retention.HistorySeconds != int64((90*24*time.Hour)/time.Second) {
		t.Fatalf("history seconds = %d, want 7776000", got.Retention.HistorySeconds)
	}
	if got.SiteName != "Platform Monitor" || got.SiteURL != "https://monitor.example.test" {
		t.Fatalf("runtime branding = %q %q, want configured site name and url", got.SiteName, got.SiteURL)
	}
}

// TestDashboardResponseShapeIncludesRuntimeConfig verifies the dashboard payload exposes runtime metadata.
func TestDashboardResponseShapeIncludesRuntimeConfig(t *testing.T) {
	nextCheck := time.Date(2026, 5, 17, 10, 15, 0, 0, time.UTC)
	payload := DashboardResponse{
		GeneratedAt: time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC),
		Models: []store.ModelState{{
			ModelID:     "chat-a",
			Capability:  "chat",
			Status:      "active",
			NextCheckAt: &nextCheck,
		}},
		Config: RuntimeConfig{
			Retention: RetentionRuntimeConfig{HistorySeconds: 42},
			SiteName:  "Platform Monitor",
			SiteURL:   "https://monitor.example.test",
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got struct {
		Config struct {
			Retention struct {
				HistorySeconds int64 `json:"history_seconds"`
			} `json:"retention"`
			SiteName string `json:"site_name"`
			SiteURL  string `json:"site_url"`
		} `json:"config"`
		Models []struct {
			ModelID     string `json:"model_id"`
			NextCheckAt string `json:"next_check_at"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(got.Models) != 1 || got.Models[0].ModelID != "chat-a" || got.Models[0].NextCheckAt == "" {
		t.Fatalf("models = %#v, want serialized next_check_at", got.Models)
	}
	if got.Config.Retention.HistorySeconds != 42 {
		t.Fatalf("history seconds = %d, want 42", got.Config.Retention.HistorySeconds)
	}
	if got.Config.SiteName != "Platform Monitor" || got.Config.SiteURL != "https://monitor.example.test" {
		t.Fatalf("runtime branding = %q %q, want configured site name and url", got.Config.SiteName, got.Config.SiteURL)
	}
}

type scheduledManualQueue struct {
	manualChecksQueue
	next map[string]time.Time
	err  error
}

func (q *scheduledManualQueue) ScheduledModelRuns(context.Context) (map[string]time.Time, error) {
	return q.next, q.err
}

func TestAttachModelNextChecksUsesQueueSchedule(t *testing.T) {
	nextCheck := time.Date(2026, 5, 17, 10, 15, 0, 0, time.UTC)
	router := Router{taskQueue: &scheduledManualQueue{
		next: map[string]time.Time{"chat-a": nextCheck},
	}}

	models := router.attachModelNextChecks(context.Background(), []store.ModelState{
		{ModelID: "chat-a", Capability: "chat", Status: "active"},
		{ModelID: "inactive", Capability: "chat", Status: "inactive"},
	})

	if models[0].NextCheckAt == nil || !models[0].NextCheckAt.Equal(nextCheck) {
		t.Fatalf("chat next_check_at = %v, want %s", models[0].NextCheckAt, nextCheck)
	}
	if models[1].NextCheckAt != nil {
		t.Fatalf("inactive next_check_at = %v, want nil", models[1].NextCheckAt)
	}
}

// TestModelEventsResponseShape verifies the JSON contract for model event pages.
func TestModelEventsResponseShape(t *testing.T) {
	payload := ModelEventsResponse{
		ModelID: "test-model",
		Events:  []store.RecentEvent{{ID: 1, ModelID: "test-model", EventType: "removed", Changed: true}},
		Total:   42,
		Limit:   25,
		Offset:  50,
		Filters: store.ModelEventFilterOptions{
			Statuses:   []string{"ok"},
			Sources:    []string{"scheduler"},
			EventTypes: []string{"scheduled_run"},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got struct {
		ModelID string `json:"model_id"`
		Events  []struct {
			Changed bool `json:"changed"`
		} `json:"events"`
		Total   int64 `json:"total"`
		Limit   int   `json:"limit"`
		Offset  int   `json:"offset"`
		Filters struct {
			Statuses   []string `json:"statuses"`
			Sources    []string `json:"sources"`
			EventTypes []string `json:"event_types"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.ModelID != payload.ModelID || got.Total != payload.Total || got.Limit != payload.Limit || got.Offset != payload.Offset {
		t.Fatalf("response metadata = %#v, want %#v", got, payload)
	}
	if len(got.Events) != 1 || !got.Events[0].Changed {
		t.Fatalf("event changed = %#v, want one changed event", got.Events)
	}
	assertStrings(t, got.Filters.Statuses, []string{"ok"})
	assertStrings(t, got.Filters.Sources, []string{"scheduler"})
	assertStrings(t, got.Filters.EventTypes, []string{"scheduled_run"})
}

// TestModelDashboardResponseShape verifies the JSON contract for model detail payloads.
func TestModelDashboardResponseShape(t *testing.T) {
	fixturePath := "/config/embedding-fixture.txt"
	fixtureBytes := 128
	vectorDimensions := 1536
	payload := ModelDashboardResponse{
		GeneratedAt: time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC),
		Model:       store.ModelState{ModelID: "test-model", Capability: "embedding", Status: "active"},
		KPIs:        store.KPISummary{TotalRuns: 2, SuccessRate: 1},
		SLO:         store.SLOThresholds{TTFTP99MS: 1000},
		Charts:      []ChartResponse{{ID: "model-vector-dimensions", Title: "Vector dimensions", Type: "bar", Metric: "vector_dimensions"}},
		Runs: []store.RecentRun{{
			Capability:       "embedding",
			ModelID:          "test-model",
			FixturePath:      &fixturePath,
			FixtureBytes:     &fixtureBytes,
			VectorDimensions: &vectorDimensions,
		}},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got struct {
		Model struct {
			ModelID string `json:"model_id"`
		} `json:"model"`
		KPIs struct {
			TotalRuns int64 `json:"total_runs"`
		} `json:"kpis"`
		Charts []any `json:"charts"`
		Runs   []struct {
			Capability       string `json:"capability"`
			FixturePath      string `json:"fixture_path"`
			FixtureBytes     int    `json:"fixture_bytes"`
			VectorDimensions int    `json:"vector_dimensions"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Model.ModelID != "test-model" || got.KPIs.TotalRuns != 2 || len(got.Charts) != 1 || len(got.Runs) != 1 {
		t.Fatalf("response shape = %#v, want model, kpis, charts, and runs", got)
	}
	if got.Runs[0].Capability != "embedding" || got.Runs[0].FixturePath != fixturePath || got.Runs[0].FixtureBytes != fixtureBytes || got.Runs[0].VectorDimensions != vectorDimensions {
		t.Fatalf("embedding run fields = %#v, want fixture and vector metadata", got.Runs[0])
	}
}

type chartMetricStore struct {
	metricsFakeStore
	samples  map[string][]store.MetricSample
	errors   map[string]error
	requests []string
}

func (f *chartMetricStore) MetricSamples(_ context.Context, metric, groupBy string, _ time.Time) ([]store.MetricSample, error) {
	key := metric + "|" + groupBy
	f.requests = append(f.requests, key)
	if err := f.errors[key]; err != nil {
		return nil, err
	}
	return f.samples[key], nil
}

// assertStrings compares two string slices in order.
func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(%v) = %d, want %d", got, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("values = %v, want %v", got, want)
		}
	}
}

// assertChartTypes compares configured chart types by chart ID.
func assertChartTypes(t *testing.T, got, want map[string]string) {
	t.Helper()
	for id, wantType := range want {
		if got[id] != wantType {
			t.Fatalf("chart %q type = %q, want %q", id, got[id], wantType)
		}
	}
}

// assertChartData compares nullable chart values in order.
func assertChartData(t *testing.T, got, want []*float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("data len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if want[i] == nil {
			if got[i] != nil {
				t.Fatalf("bucket %d = %v, want nil", i, *got[i])
			}
			continue
		}
		if got[i] == nil || *got[i] != *want[i] {
			var gotValue any
			if got[i] != nil {
				gotValue = *got[i]
			}
			t.Fatalf("bucket %d = %v, want %v", i, gotValue, *want[i])
		}
	}
}
