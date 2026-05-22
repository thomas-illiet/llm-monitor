package queue

import (
	"context"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewModelSnapshotTask creates the Asynq task for model inventory refreshes.
func NewModelSnapshotTask(providerID string) *asynq.Task {
	return asynq.NewTask(shared.ModelSnapshotTaskName, shared.MarshalProviderTaskPayload(providerID))
}

// EnqueueModelSnapshot enqueues a manual model inventory refresh.
func (c *Client) EnqueueModelSnapshot(ctx context.Context, providerID string) (EnqueuedTask, error) {
	return c.enqueue(ctx, NewModelSnapshotTask(providerID), providerID, "")
}
