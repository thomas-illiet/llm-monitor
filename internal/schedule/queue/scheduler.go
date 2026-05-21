package queue

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
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
	for _, model := range models {
		task, err := NewScheduledModelRunTask(model)
		if err != nil {
			return nil, err
		}
		configs = append(configs, &asynq.PeriodicTaskConfig{
			Cronspec: every(p.modelInterval(model.ModelID)),
			Task:     task,
			Opts:     taskOptions(p.cfg),
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
		if wildcardMatch(pattern, modelID) {
			return override.Interval.Duration
		}
	}
	return p.cfg.Schedules.ModelRuns.Duration
}

func every(interval time.Duration) string {
	return fmt.Sprintf("@every %s", interval)
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

func wildcardMatch(pattern, value string) bool {
	regex := wildcardPatternRegexp(pattern)
	matched, err := regexp.MatchString(regex, value)
	return err == nil && matched
}

func wildcardPatternRegexp(pattern string) string {
	var builder strings.Builder
	builder.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			builder.WriteString(".*")
		case '?':
			builder.WriteString(".")
		default:
			builder.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	builder.WriteString("$")
	return builder.String()
}
