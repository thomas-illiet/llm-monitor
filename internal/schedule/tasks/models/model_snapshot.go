package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

// NewModelSnapshotTask creates the model inventory snapshot task.
func NewModelSnapshotTask(deps shared.Dependencies) runner.Task {
	service := newService(deps)
	return runner.Task{
		Name:    shared.ModelSnapshotTaskName,
		Handler: service.refreshModels,
	}
}

func (s *service) refreshModels(ctx context.Context, taskCtx runner.TaskContext) error {
	return s.refreshProviders(ctx, &taskCtx)
}

func (s *service) refreshProviders(ctx context.Context, taskCtx *runner.TaskContext) error {
	var requested string
	if taskCtx != nil {
		payload, err := shared.UnmarshalProviderTaskPayload(taskCtx.Payload)
		if err != nil {
			return err
		}
		requested = payload.ProviderID
	}
	var joined error
	for _, providerID := range providerIDs(s.client.ProviderIDs(), requested) {
		if err := s.refreshProviderModels(ctx, providerID); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func (s *service) refreshProviderModels(ctx context.Context, providerID string) error {
	providerModels, err := s.client.ListModels(ctx, providerID)
	if err != nil {
		s.logger.Error("list models", "provider", providerID, "error", err)
		now := time.Now().UTC()
		markErr := s.markAllModelsInactive(ctx, providerID, now, "inventory", "model inventory request failed: "+err.Error())
		if markErr != nil {
			return errors.Join(err, markErr)
		}
		s.sendInactiveModelAlerts(ctx, now)
		return err
	}
	s.logger.Debug("model inventory loaded", "provider", providerID, "models", len(providerModels))
	knownCapabilities := s.lastKnownRunnableCapabilities(ctx, providerID)
	observed := s.detectModels(ctx, providerID, providerModels, knownCapabilities)
	s.logger.Debug("model capability detection completed", "provider", providerID, "models", len(observed))
	now := time.Now().UTC()
	events, err := s.store.ProcessModelObservation(ctx, providerID, observed, now)
	if err != nil {
		s.logger.Error("process model observation", "error", err)
		return err
	}
	s.reloadProviderModelPlan(providerID, observed)
	for _, event := range events {
		switch event.EventType {
		case "added":
			if event.FirstSeen {
				body := fmt.Sprintf("Model %s appeared for the first time at %s.", event.ModelID, event.ObservedAt.Format(time.RFC3339))
				s.sendModelAlert(ctx, modelAlertKey("first-seen", event.ProviderID, event.ModelID, event.ObservedAt), event.ProviderID, event.ModelID, "first_seen", "New LLM model detected", body, s.modelAlertFields(event.ProviderID, event.ModelID,
					notify.AlertField{Label: "Detected at", Value: event.ObservedAt.Format(time.RFC3339)},
				))
			}
		case "returned":
			if shouldAlertReturned(event.MissingDuration, s.cfg.Models.AbsenceAlertAfter.Duration) {
				body := fmt.Sprintf("Model %s returned at %s after being absent for %s.", event.ModelID, event.ObservedAt.Format(time.RFC3339), formatAlertDuration(event.MissingDuration))
				s.sendModelAlert(ctx, modelAlertKey("returned", event.ProviderID, event.ModelID, event.ObservedAt), event.ProviderID, event.ModelID, "returned", "LLM model returned", body, s.modelAlertFields(event.ProviderID, event.ModelID,
					notify.AlertField{Label: "Returned at", Value: event.ObservedAt.Format(time.RFC3339)},
					notify.AlertField{Label: "Absent for", Value: formatAlertDuration(event.MissingDuration)},
				))
			}
		}
	}
	s.sendInactiveModelAlerts(ctx, now)
	return nil
}

func (s *service) lastKnownRunnableCapabilities(ctx context.Context, providerID string) map[string]string {
	if s.store == nil {
		return nil
	}
	capabilities, err := s.store.LastRunnableCapabilities(ctx, providerID)
	if err != nil {
		s.logger.Error("load last runnable capabilities", "error", err)
		return nil
	}
	return capabilities
}

func (s *service) detectModels(ctx context.Context, providerID string, providerModels []llm.ProviderModel, knownCapabilities map[string]string) []store.ObservedModel {
	observed := make([]store.ObservedModel, len(providerModels))
	if len(providerModels) == 0 {
		return observed
	}
	embeddingInput := s.embeddingProbeInput()
	sem := make(chan struct{}, s.modelConcurrency())
	var wg sync.WaitGroup
	for i, providerModel := range providerModels {
		i, providerModel := i, providerModel
		wg.Add(1)
		go func() {
			defer wg.Done()
			modelID := providerModel.ID
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				observed[i] = store.ObservedModel{
					ProviderID:       providerID,
					ID:               modelID,
					Capability:       capabilitySkip,
					SkipReason:       "context canceled before capability probes",
					ProviderMetadata: providerModel.Metadata,
				}
				s.recordModelEvent(ctx, store.ModelEventRecord{
					ProviderID: providerID,
					ModelID:    modelID,
					EventType:  "capability_probe",
					Source:     "capability_probe",
					Severity:   "warning",
					Status:     "skipped",
					Capability: capabilitySkip,
					Title:      "Capability probe skipped",
					Message:    "Capability detection was skipped because the task context was canceled.",
					Details:    map[string]any{"skip_reason": "context canceled before capability probes"},
				})
				return
			}
			detection := s.detectModelCapabilityDetails(ctx, providerID, modelID, embeddingInput)
			capability := detection.Capability
			skipReason := detection.SkipReason
			if preserved := preservedRunnableCapability(detection, knownCapabilities[modelID]); preserved != "" {
				capability = preserved
				skipReason = ""
				detection.ProbeDetails["preserved_capability"] = preserved
			}
			observed[i] = store.ObservedModel{
				ProviderID:       providerID,
				ID:               modelID,
				Capability:       capability,
				SkipReason:       skipReason,
				ProbeDetails:     detection.ProbeDetails,
				ProviderMetadata: providerModel.Metadata,
			}
			s.recordCapabilityProbeEvent(ctx, providerID, modelID, detection)
		}()
	}
	wg.Wait()
	return observed
}

func (s *service) embeddingProbeInput() string {
	if input := strings.TrimSpace(s.loadEmbeddingFixture()); input != "" {
		return input
	}
	return defaultEmbeddingProbeInput
}

func (s *service) modelConcurrency() int {
	if s.cfg.Models.MaxConcurrency > 0 {
		return s.cfg.Models.MaxConcurrency
	}
	return 1
}

func providerIDs(all []string, requested string) []string {
	if requested == "" {
		return all
	}
	return []string{requested}
}

func (s *service) reloadProviderModelPlan(providerID string, observed []store.ObservedModel) {
	next := buildModelPlan(observed)
	current := s.modelPlan.Load()
	merged := make([]shared.ModelPlanItem, 0, len(current)+len(next))
	for _, item := range current {
		if item.ProviderID != providerID {
			merged = append(merged, item)
		}
	}
	merged = append(merged, next...)
	s.modelPlan.Store(merged)
	s.logger.Info("task model plan reloaded", "provider", providerID, "models", len(next))
}
