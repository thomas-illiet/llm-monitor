package models

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

type capabilityProbeClient struct {
	embeddingResult llm.RunResult
	chatResult      llm.RunResult
	embeddingCalls  int
	chatCalls       int
}

func (c *capabilityProbeClient) ListModels(context.Context) ([]llm.ProviderModel, error) {
	return nil, nil
}

func (c *capabilityProbeClient) HealthCheck(context.Context) llm.HTTPCheckResult {
	return llm.HTTPCheckResult{}
}

func (c *capabilityProbeClient) RunChat(context.Context, llm.ChatRequest) llm.RunResult {
	c.chatCalls++
	return c.chatResult
}

func (c *capabilityProbeClient) RunChatStream(context.Context, llm.ChatRequest) llm.RunResult {
	c.chatCalls++
	return c.chatResult
}

func (c *capabilityProbeClient) RunEmbedding(context.Context, string, string) llm.RunResult {
	c.embeddingCalls++
	return c.embeddingResult
}

func TestDetectModelCapabilityFallsBackToEmbedding(t *testing.T) {
	dimensions := 3
	client := &capabilityProbeClient{
		embeddingResult: llm.RunResult{OK: true, VectorDimensions: &dimensions},
		chatResult:      llm.RunResult{OK: false, StatusCode: http.StatusBadRequest, Error: "not a chat model"},
	}
	service := testTaskService(client)

	got := service.detectModelCapability(context.Background(), "embedding-test", "probe text")

	if got != capabilityEmbedding {
		t.Fatalf("got %q, want %q", got, capabilityEmbedding)
	}
	if client.chatCalls != 1 {
		t.Fatalf("chat calls = %d, want 1", client.chatCalls)
	}
	if client.embeddingCalls != 1 {
		t.Fatalf("embedding calls = %d, want 1", client.embeddingCalls)
	}
}

func TestDetectModelCapabilityPrefersChatForGeneralModels(t *testing.T) {
	dimensions := 3
	client := &capabilityProbeClient{
		embeddingResult: llm.RunResult{OK: true, VectorDimensions: &dimensions},
		chatResult:      llm.RunResult{OK: true},
	}
	service := testTaskService(client)

	got := service.detectModelCapability(context.Background(), "smollm2:135m", "probe text")

	if got != capabilityChat {
		t.Fatalf("got %q, want %q", got, capabilityChat)
	}
	if client.chatCalls != 1 {
		t.Fatalf("chat calls = %d, want 1", client.chatCalls)
	}
	if client.embeddingCalls != 0 {
		t.Fatalf("embedding calls = %d, want 0", client.embeddingCalls)
	}
}

func TestDetectModelCapabilitySkipsWhenBothProbesFail(t *testing.T) {
	client := &capabilityProbeClient{
		embeddingResult: llm.RunResult{OK: false, Error: "not an embedding model"},
		chatResult:      llm.RunResult{OK: false, Error: "not a chat model"},
	}
	service := testTaskService(client)

	got := service.detectModelCapability(context.Background(), "audio-test", "probe text")

	if got != capabilitySkip {
		t.Fatalf("got %q, want %q", got, capabilitySkip)
	}
	if client.embeddingCalls != 1 {
		t.Fatalf("embedding calls = %d, want 1", client.embeddingCalls)
	}
	if client.chatCalls != 1 {
		t.Fatalf("chat calls = %d, want 1", client.chatCalls)
	}
}

func TestDetectModelCapabilityDetailsIncludesSkipReason(t *testing.T) {
	client := &capabilityProbeClient{
		embeddingResult: llm.RunResult{OK: false, StatusCode: 404, Error: "not embeddings"},
		chatResult:      llm.RunResult{OK: false, StatusCode: 400, Error: "not chat"},
	}
	service := testTaskService(client)

	got := service.detectModelCapabilityDetails(context.Background(), "audio-test", "probe text")

	if got.Capability != capabilitySkip {
		t.Fatalf("capability = %q, want %q", got.Capability, capabilitySkip)
	}
	if got.SkipReason == "" {
		t.Fatal("expected a verbose skip reason")
	}
	if got.ProbeDetails["embedding"] == nil || got.ProbeDetails["chat"] == nil {
		t.Fatalf("expected embedding and chat details, got %#v", got.ProbeDetails)
	}
}

func TestDetectModelCapabilityUsesUnknownForTransientProbeFailure(t *testing.T) {
	client := &capabilityProbeClient{
		embeddingResult: llm.RunResult{OK: false, StatusCode: http.StatusNotFound, Error: "Cannot POST /v1/embeddings"},
		chatResult:      llm.RunResult{OK: false, StatusCode: http.StatusTooManyRequests, Error: "All models exhausted. Add more API keys or wait for rate limits to reset."},
	}
	service := testTaskService(client)

	got := service.detectModelCapabilityDetails(context.Background(), "rate-limited-model", "probe text")

	if got.Capability != capabilityUnknown {
		t.Fatalf("capability = %q, want %q", got.Capability, capabilityUnknown)
	}
	if got.SkipReason == "" {
		t.Fatal("expected a verbose transient probe reason")
	}
	if got.ProbeDetails["selected_capability"] != capabilityUnknown {
		t.Fatalf("selected capability = %#v, want %q", got.ProbeDetails["selected_capability"], capabilityUnknown)
	}
	if got.ProbeDetails["probe_status"] != "transient_error" {
		t.Fatalf("probe status = %#v, want transient_error", got.ProbeDetails["probe_status"])
	}
}

func TestDetectModelsPreservesKnownCapabilityForGatewayBadRequest(t *testing.T) {
	client := &capabilityProbeClient{
		embeddingResult: llm.RunResult{OK: false, StatusCode: http.StatusBadRequest, Error: "llm gateway upstream failed while routing request"},
		chatResult:      llm.RunResult{OK: false, StatusCode: http.StatusBadRequest, Error: "llm gateway upstream failed while routing request"},
	}
	service := testTaskService(client)

	got := service.detectModels(context.Background(), []llm.ProviderModel{{
		ID:       "known-chat",
		Metadata: map[string]any{"owned_by": "acme"},
	}}, map[string]string{"known-chat": capabilityChat})

	if len(got) != 1 {
		t.Fatalf("observed models = %d, want 1", len(got))
	}
	if got[0].Capability != capabilityChat {
		t.Fatalf("capability = %q, want preserved %q", got[0].Capability, capabilityChat)
	}
	if got[0].SkipReason != "" {
		t.Fatalf("skip reason = %q, want empty after capability preservation", got[0].SkipReason)
	}
	if got[0].ProbeDetails["preserved_capability"] != capabilityChat {
		t.Fatalf("preserved capability detail = %#v, want %q", got[0].ProbeDetails["preserved_capability"], capabilityChat)
	}
	if got[0].ProviderMetadata["owned_by"] != "acme" {
		t.Fatalf("provider metadata = %#v, want preserved metadata", got[0].ProviderMetadata)
	}
}

func TestPreservedRunnableCapabilityKeepsKnownCapabilityForTransientFailure(t *testing.T) {
	detection := capabilityDetection{Capability: capabilityUnknown, SkipReason: "chat probe temporarily unavailable"}

	got := preservedRunnableCapability(detection, capabilityChat)

	if got != capabilityChat {
		t.Fatalf("got %q, want %q", got, capabilityChat)
	}
}

func TestPreservedRunnableCapabilityDoesNotMaskPermanentSkip(t *testing.T) {
	detection := capabilityDetection{Capability: capabilitySkip, SkipReason: "embedding and chat capability probes failed"}

	got := preservedRunnableCapability(detection, capabilityChat)

	if got != "" {
		t.Fatalf("got preserved capability %q, want empty", got)
	}
}

func TestRunDetailsIncludesStreamingMetrics(t *testing.T) {
	ttft := 25_000_000
	itl := 10_000_000
	tpot := 9_000_000
	request := 120_000_000
	outputRate := 42.5
	inputTokens := 12
	outputTokens := 5
	result := llm.RunResult{
		OK:                    true,
		StatusCode:            200,
		Latency:               120_000_000,
		TTFT:                  durationPtrForTest(ttft),
		ITL:                   durationPtrForTest(itl),
		TPOT:                  durationPtrForTest(tpot),
		RequestLatency:        durationPtrForTest(request),
		InputTokens:           &inputTokens,
		OutputTokens:          &outputTokens,
		OutputTokensPerSecond: &outputRate,
	}

	details := runDetails(result)

	if details["ttft_ms"] != float64(25) || details["itl_ms"] != float64(10) || details["tpot_ms"] != float64(9) {
		t.Fatalf("missing streaming details: %#v", details)
	}
	if details["output_tokens_per_second"] != outputRate {
		t.Fatalf("output rate = %#v", details["output_tokens_per_second"])
	}
}

func TestFormatAlertDuration(t *testing.T) {
	got := formatAlertDuration((2 * time.Hour) + (3 * time.Minute) + (4 * time.Second))
	if got != "2h 3m 4s" {
		t.Fatalf("duration = %q, want 2h 3m 4s", got)
	}
}

func TestProbeFailureSummaryTruncatesLongErrors(t *testing.T) {
	got := probeFailureSummary(llm.RunResult{StatusCode: http.StatusTooManyRequests, Error: strings.Repeat("x", 300)})
	if !strings.HasPrefix(got, "HTTP 429 (") || !strings.HasSuffix(got, "...)") {
		t.Fatalf("summary = %q, want truncated HTTP summary", got)
	}
}

func TestModelUnavailableResultDetectsRemovedModel(t *testing.T) {
	tests := []llm.RunResult{
		{OK: false, StatusCode: http.StatusNotFound, Error: "llm returned 404: model not found"},
		{OK: false, StatusCode: http.StatusBadRequest, Error: "model_not_found: no such model"},
		{OK: false, StatusCode: http.StatusUnprocessableEntity, Error: "model does not exist"},
		{OK: false, StatusCode: http.StatusServiceUnavailable, Error: "503 Model unavailable"},
	}
	for _, tt := range tests {
		if !isModelUnavailableResult(tt) {
			t.Fatalf("result %#v should be model unavailable", tt)
		}
	}
	if isModelUnavailableResult(llm.RunResult{OK: false, StatusCode: http.StatusServiceUnavailable, Error: "temporarily unavailable"}) {
		t.Fatal("temporary outage should not be classified as removed model")
	}
}

func durationPtrForTest(nanos int) *time.Duration {
	value := time.Duration(nanos)
	return &value
}

func testTaskService(client shared.LLMClient) *service {
	return newService(shared.Dependencies{
		Config: config.Config{
			Models: config.ModelsConfig{MaxConcurrency: 2},
		},
		Client: client,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}
