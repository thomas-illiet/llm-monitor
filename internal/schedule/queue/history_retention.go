package queue

import (
	"context"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewHistoryRetentionTask creates the Asynq task for persisted history pruning.
func NewHistoryRetentionTask() *asynq.Task {
	return asynq.NewTask(shared.HistoryRetentionTaskName, nil)
}

// EnqueueHistoryRetention enqueues a manual/startup history pruning run.
func (c *Client) EnqueueHistoryRetention(ctx context.Context) (EnqueuedTask, error) {
	return c.enqueue(ctx, NewHistoryRetentionTask(), "", "")
}
