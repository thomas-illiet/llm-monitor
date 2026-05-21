package queue

import (
	"context"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewAuthCheckTask creates the Asynq task for authentication checks.
func NewAuthCheckTask() *asynq.Task {
	return asynq.NewTask(shared.AuthCheckTaskName, nil)
}

// EnqueueAuthCheck enqueues a manual authentication check.
func (c *Client) EnqueueAuthCheck(ctx context.Context) (EnqueuedTask, error) {
	return c.enqueue(ctx, NewAuthCheckTask(), "")
}
