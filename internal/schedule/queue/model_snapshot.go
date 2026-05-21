package queue

import (
	"context"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewModelSnapshotTask creates the Asynq task for model inventory refreshes.
func NewModelSnapshotTask() *asynq.Task {
	return asynq.NewTask(shared.ModelSnapshotTaskName, nil)
}

// EnqueueModelSnapshot enqueues a manual model inventory refresh.
func (c *Client) EnqueueModelSnapshot(ctx context.Context) (EnqueuedTask, error) {
	return c.enqueue(ctx, NewModelSnapshotTask(), "")
}
