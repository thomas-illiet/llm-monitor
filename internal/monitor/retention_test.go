package monitor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
)

type retentionRepository struct {
	pruneCalls int
	cutoff     time.Time
}

func (r *retentionRepository) RecordHTTPCheck(context.Context, store.CheckRecord) error {
	return nil
}

func (r *retentionRepository) RecordAuthCheck(context.Context, store.CheckRecord) error {
	return nil
}

func (r *retentionRepository) ProcessModelObservation(context.Context, []store.ObservedModel, time.Time) ([]store.ModelEvent, error) {
	return nil, nil
}

func (r *retentionRepository) LastRunnableCapabilities(context.Context) (map[string]string, error) {
	return nil, nil
}

func (r *retentionRepository) MissingModelsForAlert(context.Context, time.Duration, time.Time) ([]store.ModelState, error) {
	return nil, nil
}

func (r *retentionRepository) EmailAlertExists(context.Context, string) (bool, error) {
	return false, nil
}

func (r *retentionRepository) RecordEmailAlert(context.Context, store.EmailAlertRecord) error {
	return nil
}

func (r *retentionRepository) RecordChatRun(context.Context, store.ChatRunRecord) error {
	return nil
}

func (r *retentionRepository) RecordEmbeddingRun(context.Context, store.EmbeddingRunRecord) error {
	return nil
}

func (r *retentionRepository) RecordModelEvent(context.Context, store.ModelEventRecord) error {
	return nil
}

func (r *retentionRepository) PruneHistoryBefore(_ context.Context, cutoff time.Time) error {
	r.pruneCalls++
	r.cutoff = cutoff
	return nil
}

// TestRunHistoryRetentionSkipsWhenDisabled verifies zero retention does not prune.
func TestRunHistoryRetentionSkipsWhenDisabled(t *testing.T) {
	repo := &retentionRepository{}
	scheduler := retentionScheduler(config.Config{}, repo)

	if err := scheduler.RunHistoryRetention(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.pruneCalls != 0 {
		t.Fatalf("prune calls = %d, want 0", repo.pruneCalls)
	}
}

// TestRunHistoryRetentionPrunesBeforeConfiguredWindow verifies retention computes a cutoff.
func TestRunHistoryRetentionPrunesBeforeConfiguredWindow(t *testing.T) {
	repo := &retentionRepository{}
	history := 90 * 24 * time.Hour
	scheduler := retentionScheduler(config.Config{
		Retention: config.RetentionConfig{History: config.Duration{Duration: history}},
	}, repo)
	before := time.Now().UTC().Add(-history)

	if err := scheduler.RunHistoryRetention(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC().Add(-history)
	if repo.pruneCalls != 1 {
		t.Fatalf("prune calls = %d, want 1", repo.pruneCalls)
	}
	if repo.cutoff.Before(before) || repo.cutoff.After(after) {
		t.Fatalf("cutoff = %s, want between %s and %s", repo.cutoff, before, after)
	}
}

func retentionScheduler(cfg config.Config, repo Repository) *Scheduler {
	return NewScheduler(cfg, repo, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
