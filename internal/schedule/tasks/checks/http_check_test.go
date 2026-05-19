package checks

import (
	"context"
	"strings"
	"testing"
	"time"

	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

type httpCheckClient struct {
	result llm.HTTPCheckResult
}

func (c httpCheckClient) ListModels(context.Context) ([]string, error) {
	return nil, nil
}

func (c httpCheckClient) HealthCheck(context.Context) llm.HTTPCheckResult {
	return c.result
}

func (c httpCheckClient) RunChat(context.Context, llm.ChatRequest) llm.RunResult {
	return llm.RunResult{}
}

func (c httpCheckClient) RunChatStream(context.Context, llm.ChatRequest) llm.RunResult {
	return llm.RunResult{}
}

func (c httpCheckClient) RunEmbedding(context.Context, string, string) llm.RunResult {
	return llm.RunResult{}
}

type httpCheckRepository struct {
	shared.Repository
	recorded  []store.CheckRecord
	markCalls int
	source    string
	reason    string
}

func (r *httpCheckRepository) RecordHTTPCheck(_ context.Context, record store.CheckRecord) error {
	r.recorded = append(r.recorded, record)
	return nil
}

func (r *httpCheckRepository) MarkAllModelsInactive(_ context.Context, _ time.Time, source, reason string) ([]store.ModelEvent, error) {
	r.markCalls++
	r.source = source
	r.reason = reason
	return nil, nil
}

func TestHTTPCheckFailureMarksModelsInactiveAndClearsPlan(t *testing.T) {
	plan := shared.NewMemoryModelPlanStore()
	plan.Store([]shared.ModelPlanItem{{ID: "gpt-test", Capability: "chat"}})
	repo := &httpCheckRepository{}
	task := NewHTTPCheckTask(shared.Dependencies{
		Store:          repo,
		Client:         httpCheckClient{result: llm.HTTPCheckResult{CheckedAt: time.Now().UTC(), StatusCode: 503, Error: "down"}},
		ModelPlanStore: plan,
	})

	if err := task.Handler(context.Background(), runner.TaskContext{}); err != nil {
		t.Fatal(err)
	}
	if len(repo.recorded) != 1 {
		t.Fatalf("recorded checks = %d, want 1", len(repo.recorded))
	}
	if repo.markCalls != 1 || repo.source != "http_check" || !strings.Contains(repo.reason, "HTTP 503") {
		t.Fatalf("inactive mark = calls:%d source:%q reason:%q", repo.markCalls, repo.source, repo.reason)
	}
	if got := plan.Load(); len(got) != 0 {
		t.Fatalf("model plan = %#v, want cleared", got)
	}
}
