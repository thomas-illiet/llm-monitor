package queue

import (
	"context"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewAuthCheckTask creates the Asynq task for authentication checks.
func NewAuthCheckTask(providerID string) *asynq.Task {
	return asynq.NewTask(shared.AuthCheckTaskName, shared.MarshalProviderTaskPayload(providerID))
}

// EnqueueAuthCheck enqueues a manual authentication check.
func (c *Client) EnqueueAuthCheck(ctx context.Context, providerID string) (EnqueuedTask, error) {
	return c.enqueue(ctx, NewAuthCheckTask(providerID), providerID, "")
}
