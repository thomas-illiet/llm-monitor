package store

import (
	"strings"
	"testing"
)

// TestMetricQuerySupportsHTTPChecks verifies HTTP check metrics use the right source columns.
func TestMetricQuerySupportsHTTPChecks(t *testing.T) {
	query, err := metricQuery("http_latency_ms", "check")
	if err != nil {
		t.Fatalf("metricQuery() error = %v", err)
	}
	if !strings.Contains(query, "FROM http_checks") {
		t.Fatalf("query = %q, want http_checks source", query)
	}
	if !strings.Contains(query, "latency_ms AS value") {
		t.Fatalf("query = %q, want latency value expression", query)
	}
}

// TestMetricQueryRejectsUnsupportedInputs verifies invalid metrics and groupings fail.
func TestMetricQueryRejectsUnsupportedInputs(t *testing.T) {
	if _, err := metricQuery("unknown_metric", "model"); err == nil {
		t.Fatal("metricQuery() accepted an unknown metric")
	}
	if _, err := metricQuery("ttft_ms", "tenant"); err == nil {
		t.Fatal("metricQuery() accepted an unknown grouping")
	}
}

// TestModelPerformanceOrderBy verifies user-provided sort values never become raw SQL.
func TestModelPerformanceOrderBy(t *testing.T) {
	tests := []struct {
		sort string
		want string
	}{
		{sort: "error_count", want: "error_count DESC, success_rate ASC, model_id ASC"},
		{sort: "success_rate", want: "success_rate DESC, error_count ASC, model_id ASC"},
		{sort: "p95_latency_ms", want: "p95_latency_ms DESC, error_count DESC, model_id ASC"},
		{sort: "model_id", want: "model_id ASC"},
		{sort: "error_count; DROP TABLE chat_runs", want: "error_count DESC, success_rate ASC, model_id ASC"},
	}
	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			if got := modelPerformanceOrderBy(tt.sort); got != tt.want {
				t.Fatalf("modelPerformanceOrderBy(%q) = %q, want %q", tt.sort, got, tt.want)
			}
		})
	}
}
