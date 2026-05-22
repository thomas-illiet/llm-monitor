package tasks

import (
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/checks"
	"llmservicemonitor/internal/schedule/tasks/models"
	"llmservicemonitor/internal/schedule/tasks/retention"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

const (
	HTTPCheckTaskName        = shared.HTTPCheckTaskName
	AuthCheckTaskName        = shared.AuthCheckTaskName
	ModelSnapshotTaskName    = shared.ModelSnapshotTaskName
	ModelRunTaskName         = shared.ModelRunTaskName
	HistoryRetentionTaskName = shared.HistoryRetentionTaskName

	HistoryRetentionInterval = shared.HistoryRetentionInterval
)

type Dependencies = shared.Dependencies
type ModelPlanItem = shared.ModelPlanItem
type ModelPlanStore = shared.ModelPlanStore
type MemoryModelPlanStore = shared.MemoryModelPlanStore

// NewMemoryModelPlanStore creates an empty in-memory model plan store.
func NewMemoryModelPlanStore() *MemoryModelPlanStore {
	return shared.NewMemoryModelPlanStore()
}

// NewRegistry registers all monitor tasks with stable names.
func NewRegistry(deps Dependencies) (*runner.Registry, error) {
	if deps.ModelPlanStore == nil {
		deps.ModelPlanStore = NewMemoryModelPlanStore()
	}
	registry := runner.NewRegistry()
	for _, task := range []runner.Task{
		checks.NewHTTPCheckTask(deps),
		checks.NewAuthCheckTask(deps),
		models.NewModelSnapshotTask(deps),
		models.NewModelRunTask(deps),
		retention.NewHistoryRetentionTask(deps),
	} {
		if err := registry.Register(task); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
