package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// LocalScheduler runs registered tasks in-process on fixed intervals.
type LocalScheduler struct {
	registry *Registry
	logger   *slog.Logger
	groups   []Group

	runSeq    uint64
	startOnce sync.Once
}

// NewLocalScheduler prepares a local scheduler for the provided task groups.
func NewLocalScheduler(registry *Registry, logger *slog.Logger, groups ...Group) *LocalScheduler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	return &LocalScheduler{
		registry: registry,
		logger:   logger,
		groups:   append([]Group(nil), groups...),
	}
}

// Start launches all configured task groups and binds their lifetime to context.
func (s *LocalScheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		for _, group := range s.groups {
			group := group
			go s.runGroup(ctx, group)
		}
	})
}

func (s *LocalScheduler) runGroup(ctx context.Context, group Group) {
	for _, invocation := range group.Startup {
		if ctx.Err() != nil {
			return
		}
		if err := s.runInvocation(ctx, invocation); err != nil {
			s.logger.Error("scheduled task failed", "task", invocation.TaskName, "group", group.Name, "error", err)
		}
	}
	for _, scheduled := range group.Recurring {
		scheduled := scheduled
		go s.loop(ctx, group.Name, scheduled)
	}
}

func (s *LocalScheduler) loop(ctx context.Context, groupName string, scheduled ScheduledTask) {
	if scheduled.Interval <= 0 {
		s.logger.Error("scheduled task interval invalid", "task", scheduled.TaskName, "group", groupName, "interval", scheduled.Interval)
		return
	}
	if scheduled.RunImmediately {
		if err := s.runInvocation(ctx, Invocation{TaskName: scheduled.TaskName, Payload: scheduled.Payload}); err != nil {
			s.logger.Error("scheduled task failed", "task", scheduled.TaskName, "group", groupName, "error", err)
		}
	}
	ticker := time.NewTicker(scheduled.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.runInvocation(ctx, Invocation{TaskName: scheduled.TaskName, Payload: scheduled.Payload}); err != nil {
				s.logger.Error("scheduled task failed", "task", scheduled.TaskName, "group", groupName, "error", err)
			}
		}
	}
}

func (s *LocalScheduler) runInvocation(ctx context.Context, invocation Invocation) error {
	if s.registry == nil {
		return fmt.Errorf("task registry is nil")
	}
	task, ok := s.registry.Get(invocation.TaskName)
	if !ok {
		return fmt.Errorf("task %q is not registered", invocation.TaskName)
	}
	maxAttempts := task.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	scheduledAt := time.Now().UTC()
	runID := s.nextRunID(task.Name, scheduledAt)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		taskCtx := TaskContext{
			TaskName:    task.Name,
			RunID:       runID,
			Attempt:     attempt,
			ScheduledAt: scheduledAt,
			Payload:     append([]byte(nil), invocation.Payload...),
		}
		runCtx := ctx
		cancel := func() {}
		if task.Timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		}
		err := task.Handler(runCtx, taskCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxAttempts {
			s.logger.Warn("scheduled task attempt failed", "task", task.Name, "attempt", attempt, "error", err)
		}
	}
	return lastErr
}

func (s *LocalScheduler) nextRunID(taskName string, scheduledAt time.Time) string {
	seq := atomic.AddUint64(&s.runSeq, 1)
	return fmt.Sprintf("%s:%d:%d", taskName, scheduledAt.UnixNano(), seq)
}
