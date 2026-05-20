package models

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

type modelRunClient struct {
	results map[string]llm.RunResult
}

func (c *modelRunClient) ListModels(context.Context) ([]string, error) {
	return nil, nil
}

func (c *modelRunClient) HealthCheck(context.Context) llm.HTTPCheckResult {
	return llm.HTTPCheckResult{}
}

func (c *modelRunClient) RunChat(_ context.Context, run llm.ChatRequest) llm.RunResult {
	return c.resultFor(run.Model)
}

func (c *modelRunClient) RunChatStream(_ context.Context, run llm.ChatRequest) llm.RunResult {
	return c.resultFor(run.Model)
}

func (c *modelRunClient) RunEmbedding(context.Context, string, string) llm.RunResult {
	return llm.RunResult{}
}

func (c *modelRunClient) resultFor(modelID string) llm.RunResult {
	result, ok := c.results[modelID]
	if !ok {
		return llm.RunResult{StartedAt: time.Now().UTC(), OK: true, StatusCode: http.StatusOK, Latency: time.Millisecond}
	}
	if result.StartedAt.IsZero() {
		result.StartedAt = time.Now().UTC()
	}
	if result.Latency == 0 {
		result.Latency = time.Millisecond
	}
	return result
}

type recordingRunRepository struct {
	noopRepository
	mu       sync.Mutex
	chatRuns []store.ChatRunRecord
}

func (r *recordingRunRepository) RecordChatRun(_ context.Context, record store.ChatRunRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chatRuns = append(r.chatRuns, record)
	return nil
}

func TestRunModelTestsRecordsHealthyModelWhenAnotherModelReturnsBadRequest(t *testing.T) {
	plan := shared.NewMemoryModelPlanStore()
	plan.Store([]shared.ModelPlanItem{
		{ID: "broken-gateway-model", Capability: capabilityChat},
		{ID: "healthy-chat-model", Capability: capabilityChat},
	})
	repo := &recordingRunRepository{}
	service := newService(shared.Dependencies{
		Config: config.Config{
			Models: config.ModelsConfig{MaxConcurrency: 1},
			Tests: config.TestsConfig{
				ChatPrompts: []config.ChatPromptConfig{
					{ID: "smoke", Prompt: "Say ok.", MaxTokens: 4},
				},
			},
		},
		Store:          repo,
		Client:         &modelRunClient{results: map[string]llm.RunResult{"broken-gateway-model": {OK: false, StatusCode: http.StatusBadRequest, Error: "llm gateway upstream failed"}}},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		ModelPlanStore: plan,
	})

	err := service.runModelTests(context.Background(), runner.TaskContext{})

	if err != nil {
		t.Fatalf("runModelTests() error = %v", err)
	}
	if len(repo.chatRuns) != 2 {
		t.Fatalf("chat runs = %d, want 2: %#v", len(repo.chatRuns), repo.chatRuns)
	}
	records := map[string]store.ChatRunRecord{}
	for _, record := range repo.chatRuns {
		records[record.ModelID] = record
	}
	if record, ok := records["broken-gateway-model"]; !ok || record.OK {
		t.Fatalf("broken model record = %#v, present = %v; want failed record", record, ok)
	}
	if record, ok := records["healthy-chat-model"]; !ok || !record.OK {
		t.Fatalf("healthy model record = %#v, present = %v; want successful record", record, ok)
	}
}
