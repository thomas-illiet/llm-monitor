package models

import (
	"fmt"
	"time"

	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

func buildModelPlan(observed []store.ObservedModel) []shared.ModelPlanItem {
	next := make([]shared.ModelPlanItem, 0, len(observed))
	for _, model := range observed {
		if model.Excluded || model.Capability == capabilitySkip || model.Capability == capabilityUnknown {
			continue
		}
		next = append(next, shared.ModelPlanItem{ProviderID: model.ProviderID, ID: model.ID, Capability: model.Capability, Excluded: model.Excluded})
	}
	return next
}

func modelAlertKey(alertType, providerID, modelID string, at time.Time) string {
	return fmt.Sprintf("%s:%s:%s:%d", alertType, providerID, modelID, at.Unix())
}

func shouldAlertReturned(missingDuration, threshold time.Duration) bool {
	return missingDuration >= threshold
}
