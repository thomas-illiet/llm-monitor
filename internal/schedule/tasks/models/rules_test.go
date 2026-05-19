package models

import (
	"testing"
	"time"

	"llmservicemonitor/internal/store"
)

func TestBuildModelPlanSkipsExcludedSkipAndUnknownCapability(t *testing.T) {
	plan := buildModelPlan([]store.ObservedModel{
		{ID: "gpt-4.1-mini", Capability: "chat"},
		{ID: "text-embedding-3-large", Capability: "embedding"},
		{ID: "deprecated-model", Capability: "chat", Excluded: true},
		{ID: "audio-only", Capability: "skip"},
		{ID: "rate-limited-model", Capability: "unknown"},
	})
	if len(plan) != 2 {
		t.Fatalf("got %d planned models, want 2", len(plan))
	}
	if plan[0].ID != "gpt-4.1-mini" || plan[1].ID != "text-embedding-3-large" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestReturnAlertThreshold(t *testing.T) {
	if shouldAlertReturned(23*time.Hour, 24*time.Hour) {
		t.Fatal("model returning before threshold should not alert")
	}
	if !shouldAlertReturned(24*time.Hour, 24*time.Hour) {
		t.Fatal("model returning at threshold should alert")
	}
}

func TestModelAlertKeyIsStable(t *testing.T) {
	at := time.Unix(42, 0).UTC()
	got := modelAlertKey("inactive", "gpt-4.1", at)
	if got != "inactive:gpt-4.1:42" {
		t.Fatalf("got %q", got)
	}
}
