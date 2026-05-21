package queue

import (
	"context"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewHTTPCheckTask creates the Asynq task for target reachability checks.
func NewHTTPCheckTask() *asynq.Task {
	return asynq.NewTask(shared.HTTPCheckTaskName, nil)
}

// EnqueueHTTPCheck enqueues a manual target reachability check.
func (c *Client) EnqueueHTTPCheck(ctx context.Context) (EnqueuedTask, error) {
	return c.enqueue(ctx, NewHTTPCheckTask(), "")
}
