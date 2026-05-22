package shared

import (
	"context"
	"encoding/json"
	"fmt"
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
	ModelRunTaskName         = "monitor.model_run"
	HistoryRetentionTaskName = "monitor.history_retention"

	HistoryRetentionInterval = 24 * time.Hour
	ModelRunSpacing          = 30 * time.Second
)

// LLMClient describes the OpenAI-compatible operations used by monitor tasks.
type LLMClient interface {
	ProviderIDs() []string
	ListModels(ctx context.Context, providerID string) ([]llm.ProviderModel, error)
	HealthCheck(ctx context.Context, providerID string) llm.HTTPCheckResult
	RunChat(ctx context.Context, providerID string, run llm.ChatRequest) llm.RunResult
	RunChatStream(ctx context.Context, providerID string, run llm.ChatRequest) llm.RunResult
	RunEmbedding(ctx context.Context, providerID, model, input string) llm.RunResult
}

// AuthProviders describes provider-scoped auth health checks.
type AuthProviders interface {
	ProviderIDs() []string
	Check(ctx context.Context, providerID string) auth.CheckResult
}

// Repository describes the persistence operations required by monitor tasks.
type Repository interface {
	RecordHTTPCheck(ctx context.Context, record store.CheckRecord) error
	LatestHTTPCheck(ctx context.Context, providerID string) (*store.CheckRecord, error)
	RecordAuthCheck(ctx context.Context, record store.CheckRecord) error
	ProcessModelObservation(ctx context.Context, providerID string, observed []store.ObservedModel, now time.Time) ([]store.ModelEvent, error)
	MarkModelInactive(ctx context.Context, providerID, modelID string, now time.Time, source, reason string) (*store.ModelEvent, error)
	MarkAllModelsInactive(ctx context.Context, providerID string, now time.Time, source, reason string) ([]store.ModelEvent, error)
	LastRunnableCapabilities(ctx context.Context, providerID string) (map[string]string, error)
	InactiveModelsForAlert(ctx context.Context, threshold time.Duration, now time.Time) ([]store.ModelState, error)
	EmailAlertExists(ctx context.Context, key string) (bool, error)
	RecordEmailAlert(ctx context.Context, record store.EmailAlertRecord) error
	RecordChatRun(ctx context.Context, record store.ChatRunRecord) error
	RecordEmbeddingRun(ctx context.Context, record store.EmbeddingRunRecord) error
	RecordModelEvent(ctx context.Context, record store.ModelEventRecord) error
	PruneHistoryBefore(ctx context.Context, cutoff time.Time) error
	ReserveTaskStart(ctx context.Context, key string, earliest time.Time, spacing time.Duration) (time.Time, error)
}

// ModelRecoveryTrigger starts model inventory and probe work after target recovery.
type ModelRecoveryTrigger interface {
	TriggerModelRecovery(ctx context.Context, providerID string) error
}

// Dependencies groups shared task dependencies.
type Dependencies struct {
	Config          config.Config
	Store           Repository
	Client          LLMClient
	Auth            AuthProviders
	Notifier        notify.Notifier
	Logger          *slog.Logger
	ModelPlanStore  ModelPlanStore
	RecoveryTrigger ModelRecoveryTrigger
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
	ProviderID string
	ID         string
	Capability string
	Excluded   bool
}

// ModelRunPayload scopes a queued probe to one runnable model.
type ModelRunPayload struct {
	ProviderID  string    `json:"provider_id"`
	ModelID     string    `json:"model_id"`
	Capability  string    `json:"capability"`
	RequestedAt time.Time `json:"requested_at"`
	Reason      string    `json:"reason,omitempty"`
}

// MarshalModelRunPayload serializes a one-model scheduled probe payload.
func MarshalModelRunPayload(payload ModelRunPayload) ([]byte, error) {
	if payload.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	if payload.ProviderID == "" {
		return nil, fmt.Errorf("provider_id is required")
	}
	if payload.Capability == "" {
		return nil, fmt.Errorf("capability is required")
	}
	return json.Marshal(payload)
}

// UnmarshalModelRunPayload parses the queued payload for a one-model probe.
func UnmarshalModelRunPayload(raw []byte) (ModelRunPayload, error) {
	var payload ModelRunPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ModelRunPayload{}, err
	}
	if payload.ModelID == "" {
		return ModelRunPayload{}, fmt.Errorf("model_id is required")
	}
	if payload.ProviderID == "" {
		return ModelRunPayload{}, fmt.Errorf("provider_id is required")
	}
	if payload.Capability == "" {
		return ModelRunPayload{}, fmt.Errorf("capability is required")
	}
	return payload, nil
}

// ProviderTaskPayload optionally scopes provider-level tasks to one provider.
type ProviderTaskPayload struct {
	ProviderID string `json:"provider_id,omitempty"`
}

// MarshalProviderTaskPayload serializes an optional provider-scoped task payload.
func MarshalProviderTaskPayload(providerID string) []byte {
	if providerID == "" {
		return nil
	}
	raw, _ := json.Marshal(ProviderTaskPayload{ProviderID: providerID})
	return raw
}

// UnmarshalProviderTaskPayload parses optional provider task payloads.
func UnmarshalProviderTaskPayload(raw []byte) (ProviderTaskPayload, error) {
	if len(raw) == 0 {
		return ProviderTaskPayload{}, nil
	}
	var payload ProviderTaskPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ProviderTaskPayload{}, err
	}
	return payload, nil
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
