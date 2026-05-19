package runner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalSchedulerRunsImmediateTask(t *testing.T) {
	var calls atomic.Int64
	registry := NewRegistry()
	mustRegister(t, registry, Task{
		Name: "monitor.immediate",
		Handler: func(context.Context, TaskContext) error {
			calls.Add(1)
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler := NewLocalScheduler(registry, testLogger(), Group{
		Name: "test",
		Recurring: []ScheduledTask{{
			TaskName:       "monitor.immediate",
			Interval:       time.Hour,
			RunImmediately: true,
		}},
	})

	scheduler.Start(ctx)

	waitFor(t, func() bool { return calls.Load() == 1 })
}

func TestLocalSchedulerDelaysFirstRun(t *testing.T) {
	var calls atomic.Int64
	registry := NewRegistry()
	mustRegister(t, registry, Task{
		Name: "monitor.delayed",
		Handler: func(context.Context, TaskContext) error {
			calls.Add(1)
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler := NewLocalScheduler(registry, testLogger(), Group{
		Name: "test",
		Recurring: []ScheduledTask{{
			TaskName: "monitor.delayed",
			Interval: 50 * time.Millisecond,
		}},
	})

	scheduler.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Fatalf("calls before first interval = %d, want 0", got)
	}
	waitFor(t, func() bool { return calls.Load() == 1 })
}

func TestLocalSchedulerLogsErrorAndContinues(t *testing.T) {
	var calls atomic.Int64
	registry := NewRegistry()
	mustRegister(t, registry, Task{
		Name: "monitor.failing",
		Handler: func(context.Context, TaskContext) error {
			calls.Add(1)
			return errors.New("boom")
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler := NewLocalScheduler(registry, testLogger(), Group{
		Name: "test",
		Recurring: []ScheduledTask{{
			TaskName:       "monitor.failing",
			Interval:       10 * time.Millisecond,
			RunImmediately: true,
		}},
	})

	scheduler.Start(ctx)

	waitFor(t, func() bool { return calls.Load() >= 2 })
}

func TestLocalSchedulerRunsStartupBeforeRecurring(t *testing.T) {
	events := make(chan string, 2)
	registry := NewRegistry()
	mustRegister(t, registry, Task{
		Name: "monitor.snapshot",
		Handler: func(context.Context, TaskContext) error {
			events <- "snapshot"
			return nil
		},
	})
	mustRegister(t, registry, Task{
		Name: "monitor.model_runs",
		Handler: func(context.Context, TaskContext) error {
			events <- "runs"
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler := NewLocalScheduler(registry, testLogger(), Group{
		Name:    "models",
		Startup: []Invocation{{TaskName: "monitor.snapshot"}},
		Recurring: []ScheduledTask{{
			TaskName:       "monitor.model_runs",
			Interval:       time.Hour,
			RunImmediately: true,
		}},
	})

	scheduler.Start(ctx)

	if got := receiveEvent(t, events); got != "snapshot" {
		t.Fatalf("first event = %q, want snapshot", got)
	}
	if got := receiveEvent(t, events); got != "runs" {
		t.Fatalf("second event = %q, want runs", got)
	}
}

func TestLocalSchedulerRunNowRunsInvocationsInOrder(t *testing.T) {
	events := make(chan string, 2)
	registry := NewRegistry()
	mustRegister(t, registry, Task{
		Name: "monitor.snapshot",
		Handler: func(context.Context, TaskContext) error {
			events <- "snapshot"
			return nil
		},
	})
	mustRegister(t, registry, Task{
		Name: "monitor.model_runs",
		Handler: func(context.Context, TaskContext) error {
			events <- "runs"
			return nil
		},
	})
	scheduler := NewLocalScheduler(registry, testLogger())

	if err := scheduler.RunNow(context.Background(),
		Invocation{TaskName: "monitor.snapshot"},
		Invocation{TaskName: "monitor.model_runs"},
	); err != nil {
		t.Fatal(err)
	}
	if got := receiveEvent(t, events); got != "snapshot" {
		t.Fatalf("first event = %q, want snapshot", got)
	}
	if got := receiveEvent(t, events); got != "runs" {
		t.Fatalf("second event = %q, want runs", got)
	}
}

func mustRegister(t *testing.T, registry *Registry, task Task) {
	t.Helper()
	if err := registry.Register(task); err != nil {
		t.Fatal(err)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.After(500 * time.Millisecond)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("condition was not met before timeout")
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

func receiveEvent(t *testing.T, events <-chan string) string {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for event")
		return ""
	}
}
