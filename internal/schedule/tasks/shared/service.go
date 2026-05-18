package shared

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"llmservicemonitor/internal/auth"
	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/store"
)

const (
	HTTPCheckTaskName        = "monitor.http_check"
	AuthCheckTaskName        = "monitor.auth_check"
	ModelSnapshotTaskName    = "monitor.model_snapshot"
	ModelRunsTaskName        = "monitor.model_runs"
	HistoryRetentionTaskName = "monitor.history_retention"

	HistoryRetentionInterval = 24 * time.Hour
)

// LLMClient describes the OpenAI-compatible operations used by monitor tasks.
type LLMClient interface {
	ListModels(ctx context.Context) ([]string, error)
	HealthCheck(ctx context.Context) llm.HTTPCheckResult
	RunChat(ctx context.Context, run llm.ChatRequest) llm.RunResult
	RunChatStream(ctx context.Context, run llm.ChatRequest) llm.RunResult
	RunEmbedding(ctx context.Context, model, input string) llm.RunResult
}

// Repository describes the persistence operations required by monitor tasks.
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
	PruneHistoryBefore(ctx context.Context, cutoff time.Time) error
}

// Dependencies groups shared task dependencies.
type Dependencies struct {
	Config         config.Config
	Store          Repository
	Client         LLMClient
	Auth           auth.Provider
	Notifier       notify.Notifier
	Logger         *slog.Logger
	ModelPlanStore ModelPlanStore
}

// ResolveLogger returns a usable logger for task packages.
func ResolveLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

// ModelPlanStore isolates the state shared by model snapshot and model runs.
type ModelPlanStore interface {
	Load() []ModelPlanItem
	Store([]ModelPlanItem)
}

// ModelPlanItem is one runnable scheduled probe selected from model inventory.
type ModelPlanItem struct {
	ID         string
	Capability string
	Excluded   bool
}

// MemoryModelPlanStore keeps the model plan in process for the local scheduler.
type MemoryModelPlanStore struct {
	value atomic.Value
}

// NewMemoryModelPlanStore creates an empty in-memory model plan store.
func NewMemoryModelPlanStore() *MemoryModelPlanStore {
	store := &MemoryModelPlanStore{}
	store.value.Store([]ModelPlanItem{})
	return store
}

// Load returns a snapshot of the current model plan.
func (s *MemoryModelPlanStore) Load() []ModelPlanItem {
	plan, _ := s.value.Load().([]ModelPlanItem)
	return append([]ModelPlanItem(nil), plan...)
}

// Store replaces the current model plan.
func (s *MemoryModelPlanStore) Store(plan []ModelPlanItem) {
	s.value.Store(append([]ModelPlanItem(nil), plan...))
}

// Milliseconds converts a duration into milliseconds with sub-millisecond precision.
func Milliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}
