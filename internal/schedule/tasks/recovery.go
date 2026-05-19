package tasks

import (
	"context"
	"sync"
)

// ModelRecoveryTrigger delegates recovery work once the local scheduler exists.
type ModelRecoveryTrigger struct {
	mu  sync.RWMutex
	run func(context.Context) error
}

// NewModelRecoveryTrigger creates a bindable recovery trigger.
func NewModelRecoveryTrigger() *ModelRecoveryTrigger {
	return &ModelRecoveryTrigger{}
}

// Bind connects the trigger to the scheduler callback that runs recovery tasks.
func (t *ModelRecoveryTrigger) Bind(run func(context.Context) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.run = run
}

// TriggerModelRecovery runs recovery work if a callback has been bound.
func (t *ModelRecoveryTrigger) TriggerModelRecovery(ctx context.Context) error {
	t.mu.RLock()
	run := t.run
	t.mu.RUnlock()
	if run == nil {
		return nil
	}
	return run(ctx)
}
