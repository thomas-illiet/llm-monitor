package api

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"llmservicemonitor/internal/store"
)

// TestModelStatusSamplesFromStates verifies status samples include active, missing, and skipped counts.
func TestModelStatusSamplesFromStates(t *testing.T) {
	at := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	samples := modelStatusSamplesFromStates([]store.ModelState{
		{ModelID: "chat", Capability: "chat", Status: "active"},
		{ModelID: "embedding", Capability: "embedding", Status: "missing"},
		{ModelID: "skipped-active", Capability: "skip", Status: "active"},
		{ModelID: "skipped-missing", Capability: "chat", Excluded: true, Status: "missing"},
	}, at)

	got := map[string]float64{}
	for _, sample := range samples {
		if !sample.At.Equal(at) {
			t.Fatalf("sample time = %s, want %s", sample.At, at)
		}
		got[sample.Group] = sample.Value
	}

	want := map[string]float64{
		"active":  2,
		"missing": 2,
		"skipped": 2,
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
	for _, chart := range dashboardCharts {
		got = append(got, chart.ID)
	}
	assertStrings(t, got, []string{
		"ttft-by-model",
		"request-latency-by-model",
		"http-latency",
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
	want := []float64{150, 300, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bucket %d = %v, want %v", i, got[i], want[i])
		}
	}
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

// TestModelEventsResponseShape verifies the JSON contract for model event pages.
func TestModelEventsResponseShape(t *testing.T) {
	payload := ModelEventsResponse{
		ModelID: "test-model",
		Events:  []store.RecentEvent{},
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
		Events  []any  `json:"events"`
		Total   int64  `json:"total"`
		Limit   int    `json:"limit"`
		Offset  int    `json:"offset"`
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
	assertStrings(t, got.Filters.Statuses, []string{"ok"})
	assertStrings(t, got.Filters.Sources, []string{"scheduler"})
	assertStrings(t, got.Filters.EventTypes, []string{"scheduled_run"})
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
