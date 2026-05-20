package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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

func (s *service) refreshModels(ctx context.Context, _ runner.TaskContext) error {
	modelIDs, err := s.client.ListModels(ctx)
	if err != nil {
		s.logger.Error("list models", "error", err)
		now := time.Now().UTC()
		markErr := s.markAllModelsInactive(ctx, now, "inventory", "model inventory request failed: "+err.Error())
		if markErr != nil {
			return errors.Join(err, markErr)
		}
		s.sendInactiveModelAlerts(ctx, now)
		return err
	}
	s.logger.Debug("model inventory loaded", "models", len(modelIDs))
	knownCapabilities := s.lastKnownRunnableCapabilities(ctx)
	observed := s.detectModels(ctx, modelIDs, knownCapabilities)
	s.logger.Debug("model capability detection completed", "models", len(observed))
	now := time.Now().UTC()
	events, err := s.store.ProcessModelObservation(ctx, observed, now)
	if err != nil {
		s.logger.Error("process model observation", "error", err)
		return err
	}
	s.reloadModelPlan(observed)
	for _, event := range events {
		switch event.EventType {
		case "added":
			if event.FirstSeen {
				body := fmt.Sprintf("Model %s appeared for the first time at %s.", event.ModelID, event.ObservedAt.Format(time.RFC3339))
				s.sendModelAlert(ctx, modelAlertKey("first-seen", event.ModelID, event.ObservedAt), event.ModelID, "first_seen", "New LLM model detected", body, s.modelAlertFields(event.ModelID,
					notify.AlertField{Label: "Detected at", Value: event.ObservedAt.Format(time.RFC3339)},
				))
			}
		case "returned":
			if shouldAlertReturned(event.MissingDuration, s.cfg.Models.AbsenceAlertAfter.Duration) {
				body := fmt.Sprintf("Model %s returned at %s after being absent for %s.", event.ModelID, event.ObservedAt.Format(time.RFC3339), formatAlertDuration(event.MissingDuration))
				s.sendModelAlert(ctx, modelAlertKey("returned", event.ModelID, event.ObservedAt), event.ModelID, "returned", "LLM model returned", body, s.modelAlertFields(event.ModelID,
					notify.AlertField{Label: "Returned at", Value: event.ObservedAt.Format(time.RFC3339)},
					notify.AlertField{Label: "Absent for", Value: formatAlertDuration(event.MissingDuration)},
				))
			}
		}
	}
	s.sendInactiveModelAlerts(ctx, now)
	return nil
}

func (s *service) lastKnownRunnableCapabilities(ctx context.Context) map[string]string {
	if s.store == nil {
		return nil
	}
	capabilities, err := s.store.LastRunnableCapabilities(ctx)
	if err != nil {
		s.logger.Error("load last runnable capabilities", "error", err)
		return nil
	}
	return capabilities
}

func (s *service) detectModels(ctx context.Context, modelIDs []string, knownCapabilities map[string]string) []store.ObservedModel {
	observed := make([]store.ObservedModel, len(modelIDs))
	if len(modelIDs) == 0 {
		return observed
	}
	embeddingInput := s.embeddingProbeInput()
	sem := make(chan struct{}, s.modelConcurrency())
	var wg sync.WaitGroup
	for i, modelID := range modelIDs {
		i, modelID := i, modelID
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				observed[i] = store.ObservedModel{ID: modelID, Capability: capabilitySkip, SkipReason: "context canceled before capability probes"}
				s.recordModelEvent(ctx, store.ModelEventRecord{
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
			detection := s.detectModelCapabilityDetails(ctx, modelID, embeddingInput)
			capability := detection.Capability
			skipReason := detection.SkipReason
			if preserved := preservedRunnableCapability(detection, knownCapabilities[modelID]); preserved != "" {
				capability = preserved
				skipReason = ""
				detection.ProbeDetails["preserved_capability"] = preserved
			}
			observed[i] = store.ObservedModel{
				ID:           modelID,
				Capability:   capability,
				SkipReason:   skipReason,
				ProbeDetails: detection.ProbeDetails,
			}
			s.recordCapabilityProbeEvent(ctx, modelID, detection)
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

func (s *service) reloadModelPlan(observed []store.ObservedModel) {
	next := buildModelPlan(observed)
	s.modelPlan.Store(next)
	s.logger.Info("task model plan reloaded", "models", len(next))
}
