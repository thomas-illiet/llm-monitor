package queue

import (
	"context"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewHTTPCheckTask creates the Asynq task for target reachability checks.
func NewHTTPCheckTask(providerID string) *asynq.Task {
	return asynq.NewTask(shared.HTTPCheckTaskName, shared.MarshalProviderTaskPayload(providerID))
}

// EnqueueHTTPCheck enqueues a manual target reachability check.
func (c *Client) EnqueueHTTPCheck(ctx context.Context, providerID string) (EnqueuedTask, error) {
	return c.enqueue(ctx, NewHTTPCheckTask(providerID), providerID, "")
}
