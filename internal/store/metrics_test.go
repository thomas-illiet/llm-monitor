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
