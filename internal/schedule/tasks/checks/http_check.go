package checks

import (
	"context"

	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

// NewHTTPCheckTask creates the lightweight target reachability task.
func NewHTTPCheckTask(deps shared.Dependencies) runner.Task {
	logger := shared.ResolveLogger(deps.Logger)
	return runner.Task{
		Name: shared.HTTPCheckTaskName,
		Handler: func(ctx context.Context, _ runner.TaskContext) error {
			result := deps.Client.HealthCheck(ctx)
			record := store.CheckRecord{
				At:         result.CheckedAt,
				OK:         result.OK,
				StatusCode: result.StatusCode,
				LatencyMS:  shared.Milliseconds(result.Latency),
				Error:      result.Error,
			}
			if err := deps.Store.RecordHTTPCheck(ctx, record); err != nil {
				logger.Error("record http check", "error", err)
				return err
			}
			if !result.OK {
				if _, err := deps.Store.MarkAllModelsInactive(ctx, result.CheckedAt, "http_check", "target HTTP check failed: "+result.FailureSummary()); err != nil {
					logger.Error("mark models inactive after http check failure", "error", err)
					return err
				}
				if deps.ModelPlanStore != nil {
					deps.ModelPlanStore.Store(nil)
				}
			}
			return nil
		},
	}
}
