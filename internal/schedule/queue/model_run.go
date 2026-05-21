package queue

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

// NewModelRunTask creates the Asynq task for a one-model probe run.
func NewModelRunTask(model store.RunnableModel, reason string) (*asynq.Task, error) {
	payload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{
		ModelID:     model.ModelID,
		Capability:  model.Capability,
		RequestedAt: time.Now().UTC(),
		Reason:      reason,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(shared.ModelRunTaskName, payload), nil
}

// NewScheduledModelRunTask creates a stable periodic model-run task config.
func NewScheduledModelRunTask(model store.RunnableModel) (*asynq.Task, error) {
	payload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{
		ModelID:    model.ModelID,
		Capability: model.Capability,
		Reason:     "scheduled",
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(shared.ModelRunTaskName, payload), nil
}

// EnqueueModelRun enqueues a manual or recovery probe for one model.
func (c *Client) EnqueueModelRun(ctx context.Context, model store.RunnableModel, reason string) (EnqueuedTask, error) {
	task, err := NewModelRunTask(model, reason)
	if err != nil {
		return EnqueuedTask{}, err
	}
	return c.enqueue(ctx, task, model.ModelID)
}
