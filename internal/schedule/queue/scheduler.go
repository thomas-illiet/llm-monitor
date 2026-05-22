package queue

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
	"llmservicemonitor/internal/wildcard"
)

// RunnableModelsStore reads active models for dynamic model-run schedules.
type RunnableModelsStore interface {
	RunnableModels(ctx context.Context) ([]store.RunnableModel, error)
}

// PeriodicConfigProvider syncs Asynq scheduler entries with current model state.
type PeriodicConfigProvider struct {
	cfg   config.Config
	store RunnableModelsStore
}

// NewPeriodicConfigProvider creates a dynamic periodic task provider.
func NewPeriodicConfigProvider(cfg config.Config, store RunnableModelsStore) *PeriodicConfigProvider {
	return &PeriodicConfigProvider{cfg: cfg, store: store}
}

// GetConfigs returns static monitor schedules plus one model-run schedule per active model.
func (p *PeriodicConfigProvider) GetConfigs() ([]*asynq.PeriodicTaskConfig, error) {
	configs := []*asynq.PeriodicTaskConfig{
		{Cronspec: every(p.cfg.Schedules.HTTPCheck.Duration), Task: NewHTTPCheckTask(), Opts: taskOptions(p.cfg)},
		{Cronspec: every(p.cfg.Schedules.AuthCheck.Duration), Task: NewAuthCheckTask(), Opts: taskOptions(p.cfg)},
		{Cronspec: every(p.cfg.Schedules.ModelSnapshot.Duration), Task: NewModelSnapshotTask(), Opts: taskOptions(p.cfg)},
	}
	if p.cfg.Retention.History.Duration > 0 {
		configs = append(configs, &asynq.PeriodicTaskConfig{
			Cronspec: every(24 * time.Hour),
			Task:     NewHistoryRetentionTask(),
			Opts:     taskOptions(p.cfg),
		})
	}
	if p.store == nil {
		return configs, nil
	}
	models, err := p.store.RunnableModels(context.Background())
	if err != nil {
		return nil, err
	}
	for i, model := range models {
		task, err := NewScheduledModelRunTask(model)
		if err != nil {
			return nil, err
		}
		configs = append(configs, &asynq.PeriodicTaskConfig{
			Cronspec: every(p.modelInterval(model.ModelID)),
			Task:     task,
			Opts:     scheduledModelRunTaskOptions(p.cfg, i),
		})
	}
	return configs, nil
}

func (p *PeriodicConfigProvider) modelInterval(modelID string) time.Duration {
	for _, override := range p.cfg.Schedules.ModelRunOverrides {
		if strings.TrimSpace(override.ModelID) == modelID && override.Interval.Duration > 0 {
			return override.Interval.Duration
		}
	}
	for _, override := range p.cfg.Schedules.ModelRunOverrides {
		pattern := strings.TrimSpace(override.Pattern)
		if pattern == "" || override.Interval.Duration <= 0 {
			continue
		}
		if wildcard.Match(pattern, modelID) {
			return override.Interval.Duration
		}
	}
	return p.cfg.Schedules.ModelRuns.Duration
}

func every(interval time.Duration) string {
	return fmt.Sprintf("@every %s", interval)
}

func scheduledModelRunTaskOptions(cfg config.Config, index int) []asynq.Option {
	options := taskOptions(cfg)
	if index < 0 {
		index = 0
	}
	options = append(options, asynq.ProcessIn(time.Duration(index)*shared.ModelRunSpacing))
	return options
}

// NewPeriodicTaskManager creates the Asynq dynamic scheduler manager.
func NewPeriodicTaskManager(cfg config.Config, provider asynq.PeriodicTaskConfigProvider, logger *slog.Logger) (*asynq.PeriodicTaskManager, error) {
	return asynq.NewPeriodicTaskManager(asynq.PeriodicTaskManagerOpts{
		PeriodicTaskConfigProvider: provider,
		RedisConnOpt:               RedisOpt(cfg),
		SchedulerOpts: &asynq.SchedulerOpts{
			Logger:   newSlogAdapter(logger),
			LogLevel: asynqLogLevel(cfg.Logging.Level),
			PostEnqueueFunc: func(info *asynq.TaskInfo, err error) {
				if logger == nil {
					return
				}
				if err != nil {
					logger.Error("scheduled task enqueue failed", "error", err)
					return
				}
				logger.Debug("scheduled task enqueued", "task", info.Type, "id", info.ID, "queue", info.Queue)
			},
		},
		SyncInterval: cfg.Asynq.SchedulerSyncInterval.Duration,
	})
}
