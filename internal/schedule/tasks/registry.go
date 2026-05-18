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
	ModelRunsTaskName        = shared.ModelRunsTaskName
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
		models.NewModelRunsTask(deps),
		retention.NewHistoryRetentionTask(deps),
	} {
		if err := registry.Register(task); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// LocalScheduleGroups returns the current in-process schedules for monitor tasks.
func LocalScheduleGroups(deps Dependencies) []runner.Group {
	cfg := deps.Config
	groups := []runner.Group{
		{
			Name: "checks",
			Recurring: []runner.ScheduledTask{
				{TaskName: HTTPCheckTaskName, Interval: cfg.Schedules.HTTPCheck.Duration, RunImmediately: true},
				{TaskName: AuthCheckTaskName, Interval: cfg.Schedules.AuthCheck.Duration, RunImmediately: true},
			},
		},
		{
			Name:    "models",
			Startup: []runner.Invocation{{TaskName: ModelSnapshotTaskName}},
			Recurring: []runner.ScheduledTask{
				{TaskName: ModelSnapshotTaskName, Interval: cfg.Schedules.ModelSnapshot.Duration},
				{TaskName: ModelRunsTaskName, Interval: cfg.Schedules.ModelRuns.Duration, RunImmediately: true},
			},
		},
	}
	if cfg.Retention.History.Duration > 0 {
		groups = append(groups, runner.Group{
			Name: "retention",
			Recurring: []runner.ScheduledTask{{
				TaskName:       HistoryRetentionTaskName,
				Interval:       HistoryRetentionInterval,
				RunImmediately: true,
			}},
		})
	}
	return groups
}
