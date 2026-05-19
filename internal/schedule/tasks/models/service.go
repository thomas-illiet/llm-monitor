package models

import (
	"context"
	"log/slog"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

const (
	capabilityChat      = "chat"
	capabilityEmbedding = "embedding"
	capabilitySkip      = "skip"
	capabilityUnknown   = "unknown"

	defaultEmbeddingProbeInput = "This short text is used to detect embedding model compatibility."
	chatProbePrompt            = "Reply with ok."
)

type service struct {
	cfg       config.Config
	store     shared.Repository
	client    shared.LLMClient
	notifier  notify.Notifier
	logger    *slog.Logger
	modelPlan shared.ModelPlanStore
}

func newService(deps shared.Dependencies) *service {
	modelPlan := deps.ModelPlanStore
	if modelPlan == nil {
		modelPlan = shared.NewMemoryModelPlanStore()
	}
	return &service{
		cfg:       deps.Config,
		store:     deps.Store,
		client:    deps.Client,
		notifier:  deps.Notifier,
		logger:    shared.ResolveLogger(deps.Logger),
		modelPlan: modelPlan,
	}
}

// capabilityDetection stores the selected capability plus probe diagnostics.
type capabilityDetection struct {
	Capability   string
	SkipReason   string
	ProbeDetails map[string]any
}

func (s *service) markAllModelsInactive(ctx context.Context, now time.Time, source, reason string) error {
	if s.store == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if _, err := s.store.MarkAllModelsInactive(ctx, now, source, reason); err != nil {
		s.logger.Error("mark all models inactive", "error", err, "source", source)
		return err
	}
	s.modelPlan.Store(nil)
	return nil
}

func (s *service) markModelInactive(ctx context.Context, modelID string, now time.Time, source, reason string) error {
	if s.store == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if _, err := s.store.MarkModelInactive(ctx, modelID, now, source, reason); err != nil {
		s.logger.Error("mark model inactive", "error", err, "model", modelID, "source", source)
		return err
	}
	s.removeModelFromPlan(modelID)
	return nil
}

func (s *service) removeModelFromPlan(modelID string) {
	current := s.modelPlan.Load()
	next := make([]shared.ModelPlanItem, 0, len(current))
	for _, item := range current {
		if item.ID != modelID {
			next = append(next, item)
		}
	}
	s.modelPlan.Store(next)
}
