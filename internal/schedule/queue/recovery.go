package queue

import (
	"context"
	"errors"
	"log/slog"
)

// ModelRecoveryTrigger enqueues model work after the target recovers.
type ModelRecoveryTrigger struct {
	client *Client
	store  RunnableModelsStore
	logger *slog.Logger
}

// NewModelRecoveryTrigger creates a queue-backed recovery trigger.
func NewModelRecoveryTrigger(client *Client, store RunnableModelsStore, logger *slog.Logger) *ModelRecoveryTrigger {
	return &ModelRecoveryTrigger{client: client, store: store, logger: logger}
}

// TriggerModelRecovery refreshes inventory and immediately probes currently runnable models.
func (t *ModelRecoveryTrigger) TriggerModelRecovery(ctx context.Context) error {
	if t == nil || t.client == nil {
		return nil
	}
	var joined error
	if _, err := t.client.EnqueueModelSnapshot(ctx); err != nil {
		joined = errors.Join(joined, err)
	}
	if t.store == nil {
		return joined
	}
	models, err := t.store.RunnableModels(ctx)
	if err != nil {
		return errors.Join(joined, err)
	}
	for _, model := range models {
		if _, err := t.client.EnqueueModelRun(ctx, model, "recovery"); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	if joined != nil && t.logger != nil {
		t.logger.Error("enqueue recovery model work", "error", joined)
	}
	return joined
}
