package models

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"llmservicemonitor/internal/llm"
)

// preservedRunnableCapability keeps a known runnable capability during transient probe failures.
func preservedRunnableCapability(detection capabilityDetection, knownCapability string) string {
	if detection.Capability != capabilityUnknown {
		return ""
	}
	if knownCapability == capabilityChat || knownCapability == capabilityEmbedding {
		return knownCapability
	}
	return ""
}

func (s *service) detectModelCapability(ctx context.Context, modelID, embeddingInput string) string {
	return s.detectModelCapabilityDetails(ctx, modelID, embeddingInput).Capability
}

func (s *service) detectModelCapabilityDetails(ctx context.Context, modelID, embeddingInput string) capabilityDetection {
	return s.detectChatFirstCapability(ctx, modelID, embeddingInput)
}

func (s *service) detectChatFirstCapability(ctx context.Context, modelID, embeddingInput string) capabilityDetection {
	chat := s.client.RunChat(ctx, llm.ChatRequest{
		Model:       modelID,
		PromptID:    "capability-probe",
		Prompt:      chatProbePrompt,
		MaxTokens:   1,
		Temperature: 0,
	})
	details := map[string]any{
		"chat": runDetails(chat),
	}
	if chat.OK {
		details["selected_capability"] = capabilityChat
		return capabilityDetection{Capability: capabilityChat, ProbeDetails: details}
	}
	embedding := s.client.RunEmbedding(ctx, modelID, embeddingInput)
	details["embedding"] = runDetails(embedding)
	if embedding.OK && embedding.VectorDimensions != nil && *embedding.VectorDimensions > 0 {
		details["selected_capability"] = capabilityEmbedding
		return capabilityDetection{Capability: capabilityEmbedding, ProbeDetails: details}
	}
	if isTransientProbeFailure(chat) || isTransientProbeFailure(embedding) {
		probeReason := transientCapabilityProbeReason(embedding, chat)
		details["selected_capability"] = capabilityUnknown
		details["probe_status"] = "transient_error"
		details["probe_error"] = probeReason
		s.logger.Debug("model capability probes temporarily unavailable", "model", modelID, "embedding_error", embedding.Error, "chat_error", chat.Error)
		return capabilityDetection{Capability: capabilityUnknown, SkipReason: probeReason, ProbeDetails: details}
	}
	skipReason := "embedding and chat capability probes failed"
	details["selected_capability"] = capabilitySkip
	details["skip_reason"] = skipReason
	s.logger.Debug("model capability probes failed", "model", modelID, "embedding_error", embedding.Error, "chat_error", chat.Error)
	return capabilityDetection{Capability: capabilitySkip, SkipReason: skipReason, ProbeDetails: details}
}

func isTransientProbeFailure(result llm.RunResult) bool {
	if result.OK {
		return false
	}
	if result.StatusCode == http.StatusTooManyRequests || result.StatusCode >= http.StatusInternalServerError {
		return true
	}
	return hasTransientProbeHint(result.Error)
}

func isModelUnavailableResult(result llm.RunResult) bool {
	if result.OK {
		return false
	}
	if result.StatusCode == http.StatusNotFound || result.StatusCode == http.StatusGone {
		return true
	}
	if result.StatusCode != http.StatusBadRequest && result.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	return hasModelUnavailableHint(result.Error)
}

func hasModelUnavailableHint(errorMessage string) bool {
	errorText := strings.ToLower(errorMessage)
	if strings.Contains(errorText, "temporarily unavailable") {
		return false
	}
	hints := []string{
		"model_not_found",
		"model not found",
		"model does not exist",
		"model no longer exists",
		"model is not available",
		"model unavailable",
		"no such model",
		"unknown model",
	}
	for _, hint := range hints {
		if strings.Contains(errorText, hint) {
			return true
		}
	}
	return false
}

func hasTransientProbeHint(errorMessage string) bool {
	errorText := strings.ToLower(errorMessage)
	transientHints := []string{
		"all models exhausted",
		"connection refused",
		"connection reset",
		"context deadline",
		"eof",
		"no such host",
		"rate limit",
		"rate-limited",
		"timeout",
		"temporarily unavailable",
	}
	for _, hint := range transientHints {
		if strings.Contains(errorText, hint) {
			return true
		}
	}
	return false
}

func transientCapabilityProbeReason(embedding, chat llm.RunResult) string {
	if isTransientProbeFailure(chat) {
		return "chat capability probe temporarily unavailable: " + probeFailureSummary(chat)
	}
	if isTransientProbeFailure(embedding) {
		return "embedding capability probe temporarily unavailable: " + probeFailureSummary(embedding)
	}
	return "capability probes temporarily unavailable"
}

func probeFailureSummary(result llm.RunResult) string {
	status := "no HTTP status"
	if result.StatusCode > 0 {
		status = fmt.Sprintf("HTTP %d", result.StatusCode)
	}
	errorText := strings.TrimSpace(result.Error)
	if errorText == "" {
		return status
	}
	if len(errorText) > 240 {
		errorText = errorText[:240] + "..."
	}
	return fmt.Sprintf("%s (%s)", status, errorText)
}
