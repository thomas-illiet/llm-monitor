package retention

import (
	"context"
	"time"

	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewHistoryRetentionTask creates the persisted-history pruning task.
func NewHistoryRetentionTask(deps shared.Dependencies) runner.Task {
	logger := shared.ResolveLogger(deps.Logger)
	return runner.Task{
		Name: shared.HistoryRetentionTaskName,
		Handler: func(ctx context.Context, _ runner.TaskContext) error {
			history := deps.Config.Retention.History.Duration
			if history <= 0 || deps.Store == nil {
				return nil
			}
			cutoff := time.Now().UTC().Add(-history)
			if err := deps.Store.PruneHistoryBefore(ctx, cutoff); err != nil {
				logger.Error("prune history", "error", err, "cutoff", cutoff)
				return err
			}
			return nil
		},
	}
}
