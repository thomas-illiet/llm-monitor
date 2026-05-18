package checks

import (
	"context"

	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

// NewAuthCheckTask creates the authentication health task.
func NewAuthCheckTask(deps shared.Dependencies) runner.Task {
	logger := shared.ResolveLogger(deps.Logger)
	return runner.Task{
		Name: shared.AuthCheckTaskName,
		Handler: func(ctx context.Context, _ runner.TaskContext) error {
			result := deps.Auth.Check(ctx)
			record := store.CheckRecord{
				At:         result.CheckedAt,
				OK:         result.OK,
				StatusCode: result.StatusCode,
				LatencyMS:  shared.Milliseconds(result.Latency),
				ExpiresAt:  result.ExpiresAt,
				Error:      result.Error,
			}
			if err := deps.Store.RecordAuthCheck(ctx, record); err != nil {
				logger.Error("record auth check", "error", err)
				return err
			}
			return nil
		},
	}
}
