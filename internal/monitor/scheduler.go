package monitor

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"llmservicemonitor/internal/auth"
	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/store"
)

const (
	capabilityChat      = "chat"
	capabilityEmbedding = "embedding"
	capabilitySkip      = "skip"
	capabilityUnknown   = "unknown"

	defaultEmbeddingProbeInput = "This short text is used to detect embedding model compatibility."
	chatProbePrompt            = "Reply with ok."
)

// LLMClient describes the OpenAI-compatible operations used by scheduler probes.
type LLMClient interface {
	ListModels(ctx context.Context) ([]string, error)
	HealthCheck(ctx context.Context) llm.HTTPCheckResult
	RunChat(ctx context.Context, run llm.ChatRequest) llm.RunResult
	RunChatStream(ctx context.Context, run llm.ChatRequest) llm.RunResult
	RunEmbedding(ctx context.Context, model, input string) llm.RunResult
}

// Repository describes the persistence operations required by the scheduler.
type Repository interface {
	RecordHTTPCheck(ctx context.Context, record store.CheckRecord) error
	RecordAuthCheck(ctx context.Context, record store.CheckRecord) error
	ProcessModelObservation(ctx context.Context, observed []store.ObservedModel, now time.Time) ([]store.ModelEvent, error)
	LastRunnableCapabilities(ctx context.Context) (map[string]string, error)
	MissingModelsForAlert(ctx context.Context, threshold time.Duration, now time.Time) ([]store.ModelState, error)
	EmailAlertExists(ctx context.Context, key string) (bool, error)
	RecordEmailAlert(ctx context.Context, record store.EmailAlertRecord) error
	RecordChatRun(ctx context.Context, record store.ChatRunRecord) error
	RecordEmbeddingRun(ctx context.Context, record store.EmbeddingRunRecord) error
	RecordModelEvent(ctx context.Context, record store.ModelEventRecord) error
}

// Scheduler coordinates recurring health checks, model snapshots, probes, and alerts.
type Scheduler struct {
	cfg      config.Config
	store    Repository
	client   LLMClient
	auth     auth.Provider
	notifier notify.Notifier
	logger   *slog.Logger

	modelPlan atomic.Value
	startOnce sync.Once
}

// modelPlanItem is one runnable scheduled probe selected from model inventory.
type modelPlanItem struct {
	ID         string
	Capability string
	Excluded   bool
}

// capabilityDetection stores the selected capability plus probe diagnostics.
type capabilityDetection struct {
	Capability   string
	SkipReason   string
	ProbeDetails map[string]any
}

// NewScheduler prepares recurring checks and model-run orchestration.
func NewScheduler(cfg config.Config, db Repository, client LLMClient, authProvider auth.Provider, notifier notify.Notifier, logger *slog.Logger) *Scheduler {
	s := &Scheduler{
		cfg:      cfg,
		store:    db,
		client:   client,
		auth:     authProvider,
		notifier: notifier,
		logger:   logger,
	}
	s.modelPlan.Store([]modelPlanItem{})
	return s
}

// Start launches all scheduler loops once and binds their lifetime to context.
func (s *Scheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		go s.loop(ctx, s.cfg.Schedules.HTTPCheck.Duration, s.RunHTTPCheck)
		go s.loop(ctx, s.cfg.Schedules.AuthCheck.Duration, s.RunAuthCheck)
		go s.loop(ctx, s.cfg.Schedules.ModelSnapshot.Duration, s.RefreshModels)
		go s.loop(ctx, s.cfg.Schedules.ModelRuns.Duration, s.RunModelTests)
	})
}

// loop runs a task immediately, then repeatedly on its configured interval.
func (s *Scheduler) loop(ctx context.Context, interval time.Duration, fn func(context.Context) error) {
	if err := fn(ctx); err != nil {
		s.logger.Error("scheduled task failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				s.logger.Error("scheduled task failed", "error", err)
			}
		}
	}
}
