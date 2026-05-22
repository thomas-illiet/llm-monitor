package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/store"
)

type manualChecksStore struct {
	metricsFakeStore
	runnable []store.RunnableModel
}

func (s *manualChecksStore) RunnableModels(context.Context) ([]store.RunnableModel, error) {
	return s.runnable, nil
}

type manualChecksQueue struct {
	jobs []queue.EnqueuedTask
}

func (q *manualChecksQueue) EnqueueHTTPCheck(_ context.Context, providerID string) (queue.EnqueuedTask, error) {
	return q.add("monitor.http_check", providerID, ""), nil
}

func (q *manualChecksQueue) EnqueueAuthCheck(_ context.Context, providerID string) (queue.EnqueuedTask, error) {
	return q.add("monitor.auth_check", providerID, ""), nil
}

func (q *manualChecksQueue) EnqueueModelSnapshot(_ context.Context, providerID string) (queue.EnqueuedTask, error) {
	return q.add("monitor.model_snapshot", providerID, ""), nil
}

func (q *manualChecksQueue) EnqueueModelRun(_ context.Context, model store.RunnableModel, _ string) (queue.EnqueuedTask, error) {
	return q.add("monitor.model_run", model.ProviderID, model.ModelID), nil
}

func (q *manualChecksQueue) InspectJobs(_ context.Context, ids []string) ([]queue.JobStatus, error) {
	statuses := make([]queue.JobStatus, 0, len(ids))
	for _, id := range ids {
		statuses = append(statuses, queue.JobStatus{ID: id, Queue: "default", State: "completed"})
	}
	return statuses, nil
}

func (q *manualChecksQueue) add(taskType, providerID, modelID string) queue.EnqueuedTask {
	job := queue.EnqueuedTask{
		ID:         fmt.Sprintf("job-%d", len(q.jobs)+1),
		Queue:      "default",
		Type:       taskType,
		ProviderID: providerID,
		ModelID:    modelID,
		State:      "pending",
	}
	q.jobs = append(q.jobs, job)
	return job
}

func TestRunChecksAllEnqueuesStaticAndModelJobs(t *testing.T) {
	db := &manualChecksStore{runnable: []store.RunnableModel{
		{ProviderID: "openai", ModelID: "chat-a", Capability: "chat"},
		{ProviderID: "openai", ModelID: "embed-a", Capability: "embedding"},
	}}
	taskQueue := &manualChecksQueue{}
	router := &Router{cfg: manualChecksConfig(), store: db, taskQueue: taskQueue}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/checks/runs", strings.NewReader(`{"scope":"all"}`))
	router.runChecks(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response RunChecksResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Jobs) != 5 {
		t.Fatalf("jobs len = %d, want 5: %#v", len(response.Jobs), response.Jobs)
	}
	if response.Jobs[3].ProviderID != "openai" || response.Jobs[3].ModelID != "chat-a" || response.Jobs[4].ModelID != "embed-a" {
		t.Fatalf("model jobs = %#v, want runnable models", response.Jobs)
	}
}

func TestRunChecksModelRejectsNonRunnableModel(t *testing.T) {
	db := &manualChecksStore{runnable: []store.RunnableModel{{ProviderID: "openai", ModelID: "chat-a", Capability: "chat"}}}
	router := &Router{cfg: manualChecksConfig(), store: db, taskQueue: &manualChecksQueue{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/checks/runs", strings.NewReader(`{"scope":"model","provider_id":"openai","model_id":"missing"}`))
	router.runChecks(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func manualChecksConfig() config.Config {
	return config.Config{Providers: []config.ProviderConfig{{ID: "openai", BaseURL: "https://llm.example.test"}}}
}

func TestCheckJobsReturnsQueueStatuses(t *testing.T) {
	router := &Router{taskQueue: &manualChecksQueue{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/checks/jobs?ids=job-a,job-b", nil)
	router.checkJobs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response CheckJobsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Jobs) != 2 || response.Jobs[0].State != "completed" {
		t.Fatalf("jobs = %#v, want completed statuses", response.Jobs)
	}
}
