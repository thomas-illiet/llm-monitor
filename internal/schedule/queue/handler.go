package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

// NewServeMux maps Asynq task types to the registered monitor handlers.
func NewServeMux(registry *runner.Registry, logger *slog.Logger) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	handler := &registryHandler{registry: registry, logger: logger}
	for _, name := range registry.Names() {
		taskName := name
		mux.HandleFunc(taskName, func(ctx context.Context, task *asynq.Task) error {
			return handler.process(ctx, task)
		})
	}
	return mux
}

type registryHandler struct {
	registry *runner.Registry
	logger   *slog.Logger
}

func (h *registryHandler) process(ctx context.Context, task *asynq.Task) error {
	if h.registry == nil {
		return fmt.Errorf("task registry is nil")
	}
	registered, ok := h.registry.Get(task.Type())
	if !ok {
		return fmt.Errorf("task %q is not registered", task.Type())
	}
	taskID, ok := asynq.GetTaskID(ctx)
	if !ok {
		taskID = fmt.Sprintf("%s:%d", task.Type(), time.Now().UTC().UnixNano())
	}
	retried, _ := asynq.GetRetryCount(ctx)
	taskCtx := runner.TaskContext{
		TaskName:    task.Type(),
		RunID:       taskID,
		Attempt:     retried + 1,
		ScheduledAt: time.Now().UTC(),
		Payload:     append([]byte(nil), task.Payload()...),
	}
	startedAt := time.Now()
	if h.logger != nil {
		h.logger.Info("queued task started", taskStartLogArgs(task, taskID, taskCtx.Attempt)...)
	}
	err := registered.Handler(ctx, taskCtx)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("queued task failed", "task", task.Type(), "run", taskID, "attempt", taskCtx.Attempt, "error", err)
		}
		return err
	}
	if writer := task.ResultWriter(); writer != nil {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"completed_at": time.Now().UTC(),
			"latency_ms":   float64(time.Since(startedAt).Microseconds()) / 1000,
		})
	}
	if h.logger != nil {
		h.logger.Debug("queued task completed", "task", task.Type(), "run", taskID, "attempt", taskCtx.Attempt, "latency_ms", float64(time.Since(startedAt).Microseconds())/1000)
	}
	return nil
}

func taskStartLogArgs(task *asynq.Task, taskID string, attempt int) []any {
	args := []any{"task", task.Type(), "run", taskID, "attempt", attempt}
	if task.Type() != shared.ModelRunTaskName {
		return args
	}
	payload, err := shared.UnmarshalModelRunPayload(task.Payload())
	if err != nil {
		return args
	}
	return append(args, "model", payload.ModelID, "capability", payload.Capability, "reason", payload.Reason)
}
