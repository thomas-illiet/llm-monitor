package queue

import (
	"context"
	"testing"
	"time"

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
