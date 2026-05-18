package models

import (
	"log/slog"

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
