package monitor

import (
	"fmt"
	"time"

	"llmservicemonitor/internal/store"
)

// buildModelPlan filters observed models down to runnable scheduled jobs.
func buildModelPlan(observed []store.ObservedModel) []modelPlanItem {
	next := make([]modelPlanItem, 0, len(observed))
	for _, model := range observed {
		if model.Excluded || model.Capability == capabilitySkip || model.Capability == capabilityUnknown {
			continue
		}
		next = append(next, modelPlanItem{ID: model.ID, Capability: model.Capability, Excluded: model.Excluded})
	}
	return next
}

// modelAlertKey creates a stable deduplication key for one alert event.
func modelAlertKey(alertType, modelID string, at time.Time) string {
	return fmt.Sprintf("%s:%s:%d", alertType, modelID, at.Unix())
}

// shouldAlertReturned applies the long-absence threshold for return alerts.
func shouldAlertReturned(missingDuration, threshold time.Duration) bool {
	return missingDuration >= threshold
}
