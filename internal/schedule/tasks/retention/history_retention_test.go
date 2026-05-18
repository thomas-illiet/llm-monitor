package retention

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

type retentionRepository struct {
	shared.Repository
	pruneCalls int
	cutoff     time.Time
}

func (r *retentionRepository) PruneHistoryBefore(_ context.Context, cutoff time.Time) error {
	r.pruneCalls++
	r.cutoff = cutoff
	return nil
}

func TestHistoryRetentionTaskSkipsWhenDisabled(t *testing.T) {
	repo := &retentionRepository{}
	task := NewHistoryRetentionTask(retentionDeps(config.Config{}, repo))

	if err := task.Handler(context.Background(), runner.TaskContext{}); err != nil {
		t.Fatal(err)
	}
	if repo.pruneCalls != 0 {
		t.Fatalf("prune calls = %d, want 0", repo.pruneCalls)
	}
}

func TestHistoryRetentionTaskPrunesBeforeConfiguredWindow(t *testing.T) {
	repo := &retentionRepository{}
	history := 90 * 24 * time.Hour
	task := NewHistoryRetentionTask(retentionDeps(config.Config{
		Retention: config.RetentionConfig{History: config.Duration{Duration: history}},
	}, repo))
	before := time.Now().UTC().Add(-history)

	if err := task.Handler(context.Background(), runner.TaskContext{}); err != nil {
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

func retentionDeps(cfg config.Config, repo shared.Repository) shared.Dependencies {
	return shared.Dependencies{
		Config: cfg,
		Store:  repo,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

var _ shared.Repository = (*retentionRepository)(nil)
