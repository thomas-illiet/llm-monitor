package queue

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/schedule/tasks/shared"
	"llmservicemonitor/internal/store"
)

type periodicStore struct {
	models []store.RunnableModel
}

func (s periodicStore) RunnableModels(context.Context) ([]store.RunnableModel, error) {
	return s.models, nil
}

func TestPeriodicConfigProviderBuildsModelSchedulesWithOverrides(t *testing.T) {
	cfg := config.Config{
		Asynq: config.AsynqConfig{Queue: "monitor"},
		Schedules: config.ScheduleConfig{
			HTTPCheck:     config.Duration{Duration: 30 * time.Second},
			AuthCheck:     config.Duration{Duration: time.Minute},
			ModelSnapshot: config.Duration{Duration: 5 * time.Minute},
			ModelRuns:     config.Duration{Duration: 15 * time.Minute},
			ModelRunOverrides: []config.ModelRunScheduleOverride{
				{Pattern: "embed-*", Interval: config.Duration{Duration: 30 * time.Minute}},
				{ModelID: "embed-fast", Interval: config.Duration{Duration: 2 * time.Minute}},
			},
		},
	}
	provider := NewPeriodicConfigProvider(cfg, periodicStore{models: []store.RunnableModel{
		{ModelID: "chat-a", Capability: "chat"},
		{ModelID: "embed-a", Capability: "embedding"},
		{ModelID: "embed-fast", Capability: "embedding"},
	}})

	configs, err := provider.GetConfigs()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, cfg := range configs {
		if cfg.Task.Type() != shared.ModelRunTaskName {
			continue
		}
		payload, err := shared.UnmarshalModelRunPayload(cfg.Task.Payload())
		if err != nil {
			t.Fatal(err)
		}
		got[payload.ModelID] = cfg.Cronspec
	}
	want := map[string]string{
		"chat-a":     "@every 15m0s",
		"embed-a":    "@every 30m0s",
		"embed-fast": "@every 2m0s",
	}
	for modelID, cronspec := range want {
		if got[modelID] != cronspec {
			t.Fatalf("model %s cronspec = %q, want %q (all: %#v)", modelID, got[modelID], cronspec, got)
		}
	}
}

func TestScheduledModelRunPayloadIsStable(t *testing.T) {
	model := store.RunnableModel{ModelID: "chat-a", Capability: "chat"}
	first, err := NewScheduledModelRunTask(model)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewScheduledModelRunTask(model)
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Payload()) != string(second.Payload()) {
		t.Fatalf("scheduled payload changed between calls: %s != %s", first.Payload(), second.Payload())
	}
}

func TestModelRunNextChecksFiltersSchedulerEntries(t *testing.T) {
	chatNext := time.Date(2026, 5, 21, 10, 15, 0, 0, time.UTC)
	chatLater := chatNext.Add(5 * time.Minute)
	embedNext := time.Date(2026, 5, 21, 10, 30, 0, 0, time.UTC)
	chatPayload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{ModelID: "chat-a", Capability: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	embedPayload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{ModelID: "embed-a", Capability: "embedding"})
	if err != nil {
		t.Fatal(err)
	}

	got := modelRunNextChecks([]*asynq.SchedulerEntry{
		nil,
		{Task: nil, Next: chatNext},
		{Task: asynq.NewTask(shared.HTTPCheckTaskName, nil), Next: chatNext},
		{Task: asynq.NewTask(shared.ModelRunTaskName, []byte(`{"model_id":""}`)), Next: chatNext},
		{Task: asynq.NewTask(shared.ModelRunTaskName, chatPayload), Next: chatLater},
		{Task: asynq.NewTask(shared.ModelRunTaskName, chatPayload), Next: chatNext},
		{Task: asynq.NewTask(shared.ModelRunTaskName, embedPayload), Next: embedNext},
	})

	if len(got) != 2 {
		t.Fatalf("next checks len = %d, want 2: %#v", len(got), got)
	}
	if !got["chat-a"].Equal(chatNext) {
		t.Fatalf("chat next = %s, want %s", got["chat-a"], chatNext)
	}
	if !got["embed-a"].Equal(embedNext) {
		t.Fatalf("embed next = %s, want %s", got["embed-a"], embedNext)
	}
}
