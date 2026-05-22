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
		Handler: func(ctx context.Context, taskCtx runner.TaskContext) error {
			payload, err := shared.UnmarshalProviderTaskPayload(taskCtx.Payload)
			if err != nil {
				return err
			}
			for _, providerID := range providerIDs(deps.Auth.ProviderIDs(), payload.ProviderID) {
				result := deps.Auth.Check(ctx, providerID)
				if !result.OK {
					logger.Warn("auth check failed", "provider", providerID, "status", result.StatusCode, "latency_ms", shared.Milliseconds(result.Latency), "error", result.Error)
				} else {
					logger.Debug("auth check completed", "provider", providerID, "status", result.StatusCode, "latency_ms", shared.Milliseconds(result.Latency))
				}
				record := store.CheckRecord{
					ProviderID: providerID,
					At:         result.CheckedAt,
					OK:         result.OK,
					StatusCode: result.StatusCode,
					LatencyMS:  shared.Milliseconds(result.Latency),
					ExpiresAt:  result.ExpiresAt,
					Error:      result.Error,
				}
				if err := deps.Store.RecordAuthCheck(ctx, record); err != nil {
					logger.Error("record auth check", "provider", providerID, "error", err)
					return err
				}
			}
			return nil
		},
	}
}
