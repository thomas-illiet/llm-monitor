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
		Handler: func(ctx context.Context, taskCtx runner.TaskContext) error {
			payload, err := shared.UnmarshalProviderTaskPayload(taskCtx.Payload)
			if err != nil {
				return err
			}
			for _, providerID := range providerIDs(deps.Client.ProviderIDs(), payload.ProviderID) {
				previous, err := deps.Store.LatestHTTPCheck(ctx, providerID)
				if err != nil {
					logger.Error("load latest http check", "provider", providerID, "error", err)
				}
				result := deps.Client.HealthCheck(ctx, providerID)
				if !result.OK {
					logger.Warn("http check failed", "provider", providerID, "status", result.StatusCode, "latency_ms", shared.Milliseconds(result.Latency), "error", result.Error)
				} else {
					logger.Debug("http check completed", "provider", providerID, "status", result.StatusCode, "latency_ms", shared.Milliseconds(result.Latency))
				}
				record := store.CheckRecord{
					ProviderID: providerID,
					At:         result.CheckedAt,
					OK:         result.OK,
					StatusCode: result.StatusCode,
					LatencyMS:  shared.Milliseconds(result.Latency),
					Error:      result.Error,
				}
				if err := deps.Store.RecordHTTPCheck(ctx, record); err != nil {
					logger.Error("record http check", "provider", providerID, "error", err)
					return err
				}
				if !result.OK {
					if _, err := deps.Store.MarkAllModelsInactive(ctx, providerID, result.CheckedAt, "http_check", "provider HTTP check failed: "+result.FailureSummary()); err != nil {
						logger.Error("mark models inactive after http check failure", "provider", providerID, "error", err)
						return err
					}
					if deps.ModelPlanStore != nil {
						removeProviderFromPlan(deps.ModelPlanStore, providerID)
					}
				}
				if result.OK && previous != nil && !previous.OK && deps.RecoveryTrigger != nil {
					logger.Info("provider recovered, triggering model probes", "provider", providerID)
					if err := deps.RecoveryTrigger.TriggerModelRecovery(ctx, providerID); err != nil {
						logger.Error("trigger model recovery", "provider", providerID, "error", err)
						return err
					}
				}
			}
			return nil
		},
	}
}

func providerIDs(all []string, requested string) []string {
	if requested == "" {
		return all
	}
	return []string{requested}
}

func removeProviderFromPlan(plan shared.ModelPlanStore, providerID string) {
	current := plan.Load()
	next := make([]shared.ModelPlanItem, 0, len(current))
	for _, item := range current {
		if item.ProviderID != providerID {
			next = append(next, item)
		}
	}
	plan.Store(next)
}
