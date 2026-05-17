package monitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/store"
)

// recordCapabilityProbeEvent records the result of one capability classification probe.
func (s *Scheduler) recordCapabilityProbeEvent(ctx context.Context, modelID string, detection capabilityDetection) {
	status := "ok"
	severity := "info"
	title := "Capability probe succeeded"
	message := fmt.Sprintf("Model %s was classified as %s.", modelID, detection.Capability)
	switch detection.Capability {
	case capabilitySkip:
		status = "skipped"
		severity = "warning"
		title = "Capability probe skipped model"
		message = fmt.Sprintf("Model %s was skipped: %s.", modelID, detection.SkipReason)
	case capabilityUnknown:
		status = "error"
		severity = "error"
		title = "Capability probe temporarily unavailable"
		message = fmt.Sprintf("Model %s could not be classified yet: %s.", modelID, detection.SkipReason)
	}
	s.recordModelEvent(ctx, store.ModelEventRecord{
		ModelID:    modelID,
		EventType:  "capability_probe",
		Source:     "capability_probe",
		Severity:   severity,
		Status:     status,
		Capability: detection.Capability,
		Title:      title,
		Message:    message,
		Details: map[string]any{
			"skip_reason":   detection.SkipReason,
			"probe_details": detection.ProbeDetails,
		},
	})
}

// recordScheduledRunEvent records the event timeline entry for one scheduled probe.
func (s *Scheduler) recordScheduledRunEvent(ctx context.Context, modelID, capability, kind, promptID string, result llm.RunResult, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	for key, value := range runDetails(result) {
		details[key] = value
	}
	if promptID != "" {
		details["prompt_id"] = promptID
	}
	event := store.ModelEventRecord{
		ModelID:    modelID,
		EventType:  "scheduled_run",
		Source:     "scheduled_run",
		Severity:   "info",
		Status:     "ok",
		Capability: capability,
		Title:      fmt.Sprintf("%s probe succeeded", strings.Title(kind)),
		Message:    fmt.Sprintf("%s probe for model %s completed in %.0f ms.", strings.Title(kind), modelID, ms(result.Latency)),
		Details:    details,
	}
	if !result.OK {
		event.Severity = "error"
		event.Status = "error"
		event.Title = fmt.Sprintf("%s probe failed", strings.Title(kind))
		event.Message = fmt.Sprintf("%s probe for model %s failed: %s", strings.Title(kind), modelID, result.Error)
	}
	s.recordModelEvent(ctx, event)
}

// recordModelEvent writes a model event when storage is available.
func (s *Scheduler) recordModelEvent(ctx context.Context, record store.ModelEventRecord) {
	if s.store == nil {
		return
	}
	if err := s.store.RecordModelEvent(ctx, record); err != nil {
		s.logger.Error("record model event", "error", err, "model", record.ModelID, "event", record.EventType)
	}
}

// runDetails converts an LLM probe result into event-detail fields.
func runDetails(result llm.RunResult) map[string]any {
	details := map[string]any{
		"ok":          result.OK,
		"status_code": result.StatusCode,
		"latency_ms":  ms(result.Latency),
	}
	if result.RequestLatency != nil {
		details["request_latency_ms"] = ms(*result.RequestLatency)
	}
	if result.TTFT != nil {
		details["ttft_ms"] = ms(*result.TTFT)
	}
	if result.ITL != nil {
		details["itl_ms"] = ms(*result.ITL)
	}
	if result.TPOT != nil {
		details["tpot_ms"] = ms(*result.TPOT)
	}
	if result.InputTokens != nil {
		details["input_tokens"] = *result.InputTokens
	}
	if result.OutputTokens != nil {
		details["output_tokens"] = *result.OutputTokens
	}
	if result.TotalTokens != nil {
		details["total_tokens"] = *result.TotalTokens
	}
	if result.OutputTokensPerSecond != nil {
		details["output_tokens_per_second"] = *result.OutputTokensPerSecond
	}
	if result.VectorDimensions != nil {
		details["vector_dimensions"] = *result.VectorDimensions
	}
	if result.Error != "" {
		details["error"] = result.Error
	}
	return details
}

// msPtr converts optional durations into optional millisecond values.
func msPtr(duration *time.Duration) *float64 {
	if duration == nil {
		return nil
	}
	value := ms(*duration)
	return &value
}

// ms converts a duration into milliseconds with sub-millisecond precision.
func ms(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}
