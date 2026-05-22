package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

// NewModelRunTask creates the queued model probe task for one model.
func NewModelRunTask(deps shared.Dependencies) runner.Task {
	service := newService(deps)
	return runner.Task{
		Name:    shared.ModelRunTaskName,
		Handler: service.runModelTest,
	}
}

func (s *service) runModelTest(ctx context.Context, taskCtx runner.TaskContext) error {
	payload, err := shared.UnmarshalModelRunPayload(taskCtx.Payload)
	if err != nil {
		return fmt.Errorf("parse model run payload: %w", err)
	}
	if err := s.waitForScheduledModelSlot(ctx, payload); err != nil {
		return err
	}
	s.logger.Debug("model run probe started", "model", payload.ModelID, "capability", payload.Capability, "reason", payload.Reason)
	err = s.runOneModelTest(ctx, payload.ModelID, payload.Capability)
	if err != nil {
		s.logger.Warn("model run probe completed with errors", "model", payload.ModelID, "capability", payload.Capability, "error", err)
		return err
	}
	s.logger.Debug("model run probe completed", "model", payload.ModelID, "capability", payload.Capability)
	return nil
}

func (s *service) waitForScheduledModelSlot(ctx context.Context, payload shared.ModelRunPayload) error {
	if payload.Reason != "scheduled" || s.store == nil {
		return nil
	}
	earliest := time.Now().UTC()
	reservedAt, err := s.store.ReserveTaskStart(ctx, shared.ModelRunTaskName, earliest, shared.ModelRunSpacing)
	if err != nil {
		return fmt.Errorf("reserve model run spacing: %w", err)
	}
	wait := time.Until(reservedAt)
	if wait <= 0 {
		return nil
	}
	s.logger.Debug("model run waiting for spacing slot", "model", payload.ModelID, "reserved_at", reservedAt, "wait", wait)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *service) runOneModelTest(ctx context.Context, modelID, capability string) error {
	switch capability {
	case capabilityChat:
		return s.runChatTests(ctx, modelID)
	case capabilityEmbedding:
		return s.runEmbeddingTest(ctx, modelID, s.loadEmbeddingFixture())
	default:
		return fmt.Errorf("unsupported model capability %q for model %s", capability, modelID)
	}
}

func (s *service) runChatTests(ctx context.Context, modelID string) error {
	ran := false
	var joined error
	for _, prompt := range s.cfg.Tests.ChatPrompts {
		if prompt.ID == "" || prompt.Prompt == "" {
			continue
		}
		ran = true
		result := s.client.RunChatStream(ctx, llm.ChatRequest{
			Model:       modelID,
			PromptID:    prompt.ID,
			Prompt:      prompt.Prompt,
			MaxTokens:   prompt.MaxTokens,
			Temperature: prompt.Temperature,
		})
		record := store.ChatRunRecord{
			ModelID:               modelID,
			PromptID:              prompt.ID,
			StartedAt:             result.StartedAt,
			OK:                    result.OK,
			StatusCode:            result.StatusCode,
			LatencyMS:             ms(result.Latency),
			TTFTMS:                msPtr(result.TTFT),
			ITLMS:                 msPtr(result.ITL),
			TPOTMS:                msPtr(result.TPOT),
			RequestLatencyMS:      msPtr(result.RequestLatency),
			InputTokens:           result.InputTokens,
			OutputTokens:          result.OutputTokens,
			TotalTokens:           result.TotalTokens,
			OutputTokensPerSecond: result.OutputTokensPerSecond,
			Error:                 result.Error,
		}
		if err := s.store.RecordChatRun(ctx, record); err != nil {
			s.logger.Error("record chat run", "error", err, "model", modelID)
			joined = errors.Join(joined, err)
		}
		if !result.OK {
			s.logger.Warn("chat probe failed", "model", modelID, "prompt", prompt.ID, "status", result.StatusCode, "latency_ms", ms(result.Latency), "error", result.Error)
		} else {
			s.logger.Debug("chat probe completed", "model", modelID, "prompt", prompt.ID, "status", result.StatusCode, "latency_ms", ms(result.Latency))
		}
		s.recordScheduledRunEvent(ctx, modelID, capabilityChat, prompt.ID, result, map[string]any{
			"prompt_id":   prompt.ID,
			"max_tokens":  prompt.MaxTokens,
			"temperature": prompt.Temperature,
		})
		if isModelUnavailableResult(result) {
			if err := s.markModelInactive(ctx, modelID, result.StartedAt, "scheduled_run", "chat probe reported model unavailable: "+probeFailureSummary(result)); err != nil {
				joined = errors.Join(joined, err)
			}
			break
		}
	}
	if !ran {
		s.recordModelEvent(ctx, store.ModelEventRecord{
			ModelID:    modelID,
			EventType:  "skipped",
			Source:     "scheduled_run",
			Severity:   "warning",
			Status:     "skipped",
			Capability: capabilityChat,
			Title:      "Chat probe skipped",
			Message:    "No valid chat prompt is configured for scheduled probes.",
			Details:    map[string]any{"skip_reason": "no valid chat prompt configured"},
		})
	}
	return joined
}

func (s *service) runEmbeddingTest(ctx context.Context, modelID, input string) error {
	if strings.TrimSpace(input) == "" {
		s.logger.Warn("embedding probe skipped", "model", modelID, "fixture_path", s.cfg.Tests.EmbeddingFixture.Path, "reason", "empty embedding fixture")
		s.recordModelEvent(ctx, store.ModelEventRecord{
			ModelID:    modelID,
			EventType:  "skipped",
			Source:     "scheduled_run",
			Severity:   "warning",
			Status:     "skipped",
			Capability: capabilityEmbedding,
			Title:      "Embedding probe skipped",
			Message:    "Embedding fixture is empty or unreadable.",
			Details: map[string]any{
				"fixture_path": s.cfg.Tests.EmbeddingFixture.Path,
				"skip_reason":  "empty embedding fixture",
			},
		})
		return nil
	}
	result := s.client.RunEmbedding(ctx, modelID, input)
	record := store.EmbeddingRunRecord{
		ModelID:          modelID,
		FixturePath:      s.cfg.Tests.EmbeddingFixture.Path,
		FixtureBytes:     len([]byte(input)),
		StartedAt:        result.StartedAt,
		OK:               result.OK,
		StatusCode:       result.StatusCode,
		LatencyMS:        ms(result.Latency),
		InputTokens:      result.InputTokens,
		TotalTokens:      result.TotalTokens,
		VectorDimensions: result.VectorDimensions,
		Error:            result.Error,
	}
	if err := s.store.RecordEmbeddingRun(ctx, record); err != nil {
		s.logger.Error("record embedding run", "error", err, "model", modelID)
		return err
	}
	if !result.OK {
		s.logger.Warn("embedding probe failed", "model", modelID, "status", result.StatusCode, "latency_ms", ms(result.Latency), "error", result.Error)
	} else {
		s.logger.Debug("embedding probe completed", "model", modelID, "status", result.StatusCode, "latency_ms", ms(result.Latency))
	}
	s.recordScheduledRunEvent(ctx, modelID, capabilityEmbedding, "", result, map[string]any{
		"fixture_path":  s.cfg.Tests.EmbeddingFixture.Path,
		"fixture_bytes": len([]byte(input)),
	})
	if isModelUnavailableResult(result) {
		return s.markModelInactive(ctx, modelID, result.StartedAt, "scheduled_run", "embedding probe reported model unavailable: "+probeFailureSummary(result))
	}
	return nil
}
