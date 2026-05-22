package queue

import (
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

func TestExpiredPendingJobOnlyMatchesPendingPastDeadline(t *testing.T) {
	now := time.Date(2026, 5, 21, 20, 50, 0, 0, time.UTC)

	if !expiredPendingJob(&asynq.TaskInfo{State: asynq.TaskStatePending, Deadline: now}, now) {
		t.Fatal("pending job at deadline should be expired")
	}
	if expiredPendingJob(&asynq.TaskInfo{State: asynq.TaskStatePending, Deadline: now.Add(time.Second)}, now) {
		t.Fatal("pending job before deadline should not be expired")
	}
	if expiredPendingJob(&asynq.TaskInfo{State: asynq.TaskStateActive, Deadline: now.Add(-time.Second)}, now) {
		t.Fatal("active job past deadline should be left to worker timeout handling")
	}
}

func TestJobStatusFromTaskInfoIncludesModelRunPayload(t *testing.T) {
	payload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{
		ModelID:    "chat-a",
		Capability: "chat",
		Reason:     "manual",
	})
	if err != nil {
		t.Fatal(err)
	}

	status := jobStatusFromTaskInfo(&asynq.TaskInfo{
		ID:      "job-1",
		Queue:   "default",
		Type:    shared.ModelRunTaskName,
		Payload: payload,
		State:   asynq.TaskStatePending,
	})

	if status.ModelID != "chat-a" {
		t.Fatalf("model id = %q, want chat-a", status.ModelID)
	}
}
