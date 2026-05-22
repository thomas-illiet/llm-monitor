package queue

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

func TestRegistryHandlerLogsQueuedTaskStartAtInfo(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	registry := runner.NewRegistry()
	if err := registry.Register(runner.Task{
		Name: "monitor.example",
		Handler: func(context.Context, runner.TaskContext) error {
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	handler := registryHandler{registry: registry, logger: logger}

	if err := handler.process(context.Background(), asynq.NewTask("monitor.example", nil)); err != nil {
		t.Fatal(err)
	}

	assertLogContains(t, logs.String(),
		`level=INFO`,
		`msg="queued task started"`,
		`task=monitor.example`,
		`run=monitor.example:`,
		`attempt=1`,
	)
}

func TestRegistryHandlerLogsModelRunPayloadAtStart(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	registry := runner.NewRegistry()
	if err := registry.Register(runner.Task{
		Name: shared.ModelRunTaskName,
		Handler: func(context.Context, runner.TaskContext) error {
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	payload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{
		ProviderID: "openai",
		ModelID:    "chat-a",
		Capability: "chat",
		Reason:     "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := registryHandler{registry: registry, logger: logger}

	if err := handler.process(context.Background(), asynq.NewTask(shared.ModelRunTaskName, payload)); err != nil {
		t.Fatal(err)
	}

	assertLogContains(t, logs.String(),
		`msg="queued task started"`,
		`task=monitor.model_run`,
		`provider=openai`,
		`model=chat-a`,
		`capability=chat`,
		`reason=manual`,
	)
}

func assertLogContains(t *testing.T, log string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(log, want) {
			t.Fatalf("log = %q, want substring %q", log, want)
		}
	}
}
