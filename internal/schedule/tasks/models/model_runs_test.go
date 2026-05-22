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

func (c *modelRunClient) ListModels(context.Context) ([]llm.ProviderModel, error) {
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

type spacingRunRepository struct {
	recordingRunRepository
	reserveCalls []spacingReserveCall
}

type spacingReserveCall struct {
	key     string
	spacing time.Duration
}

func (r *spacingRunRepository) ReserveTaskStart(_ context.Context, key string, earliest time.Time, spacing time.Duration) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reserveCalls = append(r.reserveCalls, spacingReserveCall{key: key, spacing: spacing})
	return earliest, nil
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

	err := service.runModelTest(context.Background(), modelRunTaskContext(t, "broken-gateway-model", capabilityChat))
	if err != nil {
		t.Fatalf("runModelTest(broken) error = %v", err)
	}
	err = service.runModelTest(context.Background(), modelRunTaskContext(t, "healthy-chat-model", capabilityChat))

	if err != nil {
		t.Fatalf("runModelTest(healthy) error = %v", err)
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

func TestRunModelTestsRemovesServiceUnavailableModelFromPlan(t *testing.T) {
	plan := shared.NewMemoryModelPlanStore()
	plan.Store([]shared.ModelPlanItem{{ID: "unavailable-model", Capability: capabilityChat}})
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
		Client:         &modelRunClient{results: map[string]llm.RunResult{"unavailable-model": {OK: false, StatusCode: http.StatusServiceUnavailable, Error: "503 Model unavailable"}}},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		ModelPlanStore: plan,
	})

	err := service.runModelTest(context.Background(), modelRunTaskContext(t, "unavailable-model", capabilityChat))

	if err != nil {
		t.Fatalf("runModelTest() error = %v", err)
	}
	if got := plan.Load(); len(got) != 0 {
		t.Fatalf("model plan = %#v, want unavailable model removed", got)
	}
	if len(repo.chatRuns) != 1 || repo.chatRuns[0].StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("chat runs = %#v, want recorded 503 probe", repo.chatRuns)
	}
}

func TestRunModelTestReservesSpacingForScheduledRunsOnly(t *testing.T) {
	repo := &spacingRunRepository{}
	service := newService(shared.Dependencies{
		Config: config.Config{
			Tests: config.TestsConfig{
				ChatPrompts: []config.ChatPromptConfig{{ID: "smoke", Prompt: "Say ok.", MaxTokens: 4}},
			},
		},
		Store:  repo,
		Client: &modelRunClient{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if err := service.runModelTest(context.Background(), modelRunTaskContextWithReason(t, "scheduled-model", capabilityChat, "scheduled")); err != nil {
		t.Fatalf("runModelTest(scheduled) error = %v", err)
	}
	if err := service.runModelTest(context.Background(), modelRunTaskContextWithReason(t, "manual-model", capabilityChat, "manual")); err != nil {
		t.Fatalf("runModelTest(manual) error = %v", err)
	}
	if err := service.runModelTest(context.Background(), modelRunTaskContextWithReason(t, "recovery-model", capabilityChat, "recovery")); err != nil {
		t.Fatalf("runModelTest(recovery) error = %v", err)
	}

	if len(repo.reserveCalls) != 1 {
		t.Fatalf("reserve calls = %#v, want one scheduled reservation", repo.reserveCalls)
	}
	if repo.reserveCalls[0].key != shared.ModelRunTaskName || repo.reserveCalls[0].spacing != shared.ModelRunSpacing {
		t.Fatalf("reserve call = %#v, want model run spacing", repo.reserveCalls[0])
	}
}

func modelRunTaskContext(t *testing.T, modelID, capability string) runner.TaskContext {
	return modelRunTaskContextWithReason(t, modelID, capability, "")
}

func modelRunTaskContextWithReason(t *testing.T, modelID, capability, reason string) runner.TaskContext {
	t.Helper()
	payload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{
		ModelID:    modelID,
		Capability: capability,
		Reason:     reason,
	})
	if err != nil {
		t.Fatal(err)
	}
	return runner.TaskContext{Payload: payload}
}
