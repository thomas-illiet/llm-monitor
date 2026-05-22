package checks

import (
	"context"
	"errors"
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

func (c httpCheckClient) ProviderIDs() []string {
	return []string{"openai"}
}

func (c httpCheckClient) ListModels(context.Context, string) ([]llm.ProviderModel, error) {
	return nil, nil
}

func (c httpCheckClient) HealthCheck(context.Context, string) llm.HTTPCheckResult {
	return c.result
}

func (c httpCheckClient) RunChat(context.Context, string, llm.ChatRequest) llm.RunResult {
	return llm.RunResult{}
}

func (c httpCheckClient) RunChatStream(context.Context, string, llm.ChatRequest) llm.RunResult {
	return llm.RunResult{}
}

func (c httpCheckClient) RunEmbedding(context.Context, string, string, string) llm.RunResult {
	return llm.RunResult{}
}

type httpCheckRepository struct {
	shared.Repository
	recorded  []store.CheckRecord
	markCalls int
	source    string
	reason    string
	latest    *store.CheckRecord
}

func (r *httpCheckRepository) LatestHTTPCheck(context.Context, string) (*store.CheckRecord, error) {
	return r.latest, nil
}

func (r *httpCheckRepository) RecordHTTPCheck(_ context.Context, record store.CheckRecord) error {
	r.recorded = append(r.recorded, record)
	return nil
}

func (r *httpCheckRepository) MarkAllModelsInactive(_ context.Context, _ string, _ time.Time, source, reason string) ([]store.ModelEvent, error) {
	r.markCalls++
	r.source = source
	r.reason = reason
	return nil, nil
}

func TestHTTPCheckFailureMarksModelsInactiveAndClearsPlan(t *testing.T) {
	plan := shared.NewMemoryModelPlanStore()
	plan.Store([]shared.ModelPlanItem{{ProviderID: "openai", ID: "gpt-test", Capability: "chat"}})
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

type recoveryTrigger struct {
	calls int
	err   error
}

func (t *recoveryTrigger) TriggerModelRecovery(context.Context, string) error {
	t.calls++
	return t.err
}

func TestHTTPCheckRecoveryTriggersModelRecovery(t *testing.T) {
	repo := &httpCheckRepository{latest: &store.CheckRecord{OK: false}}
	recovery := &recoveryTrigger{}
	task := NewHTTPCheckTask(shared.Dependencies{
		Store:           repo,
		Client:          httpCheckClient{result: llm.HTTPCheckResult{CheckedAt: time.Now().UTC(), OK: true, StatusCode: 200}},
		RecoveryTrigger: recovery,
	})

	if err := task.Handler(context.Background(), runner.TaskContext{}); err != nil {
		t.Fatal(err)
	}
	if recovery.calls != 1 {
		t.Fatalf("recovery trigger calls = %d, want 1", recovery.calls)
	}
	if repo.markCalls != 0 {
		t.Fatalf("inactive mark calls = %d, want 0", repo.markCalls)
	}
}

func TestHTTPCheckDoesNotTriggerRecoveryWithoutFailedPreviousCheck(t *testing.T) {
	for name, previous := range map[string]*store.CheckRecord{
		"none": nil,
		"ok":   &store.CheckRecord{OK: true},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &httpCheckRepository{latest: previous}
			recovery := &recoveryTrigger{}
			task := NewHTTPCheckTask(shared.Dependencies{
				Store:           repo,
				Client:          httpCheckClient{result: llm.HTTPCheckResult{CheckedAt: time.Now().UTC(), OK: true, StatusCode: 200}},
				RecoveryTrigger: recovery,
			})

			if err := task.Handler(context.Background(), runner.TaskContext{}); err != nil {
				t.Fatal(err)
			}
			if recovery.calls != 0 {
				t.Fatalf("recovery trigger calls = %d, want 0", recovery.calls)
			}
		})
	}
}

func TestHTTPCheckRecoveryReturnsTriggerError(t *testing.T) {
	repo := &httpCheckRepository{latest: &store.CheckRecord{OK: false}}
	recoveryErr := errors.New("recovery failed")
	task := NewHTTPCheckTask(shared.Dependencies{
		Store:           repo,
		Client:          httpCheckClient{result: llm.HTTPCheckResult{CheckedAt: time.Now().UTC(), OK: true, StatusCode: 200}},
		RecoveryTrigger: &recoveryTrigger{err: recoveryErr},
	})

	if err := task.Handler(context.Background(), runner.TaskContext{}); !errors.Is(err, recoveryErr) {
		t.Fatalf("handler error = %v, want %v", err, recoveryErr)
	}
}
