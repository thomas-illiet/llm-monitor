package models

import (
	"context"
	"errors"
	"strings"
	"sync"

	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

// NewModelRunsTask creates the scheduled model probe task.
func NewModelRunsTask(deps shared.Dependencies) runner.Task {
	service := newService(deps)
	return runner.Task{
		Name:    shared.ModelRunsTaskName,
		Handler: service.runModelTests,
	}
}

func (s *service) runModelTests(ctx context.Context, _ runner.TaskContext) error {
	plan := s.modelPlan.Load()
	if len(plan) == 0 {
		s.logger.Debug("model run plan empty, skipping scheduled probes")
		return nil
	}
	s.logger.Debug("model run probes started", "models", len(plan), "concurrency", s.modelConcurrency())
	embeddingText := s.loadEmbeddingFixture()
	sem := make(chan struct{}, s.modelConcurrency())
	var wg sync.WaitGroup
	errs := make(chan error, len(plan))
	for _, model := range plan {
		model := model
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			switch model.Capability {
			case capabilityChat:
				if err := s.runChatTests(ctx, model.ID); err != nil {
					errs <- err
				}
			case capabilityEmbedding:
				if err := s.runEmbeddingTest(ctx, model.ID, embeddingText); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	var joined error
	for err := range errs {
		joined = errors.Join(joined, err)
	}
	if joined != nil {
		s.logger.Warn("model run probes completed with errors", "models", len(plan), "error", joined)
	} else {
		s.logger.Debug("model run probes completed", "models", len(plan))
	}
	return joined
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
