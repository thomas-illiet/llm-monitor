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
		{ProviderID: "openai", ModelID: "chat-a", Capability: "chat"},
		{ProviderID: "openai", ModelID: "embed-a", Capability: "embedding"},
		{ProviderID: "openai", ModelID: "embed-fast", Capability: "embedding"},
	}})

	configs, err := provider.GetConfigs()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	gotOffset := map[string]time.Duration{}
	for _, cfg := range configs {
		if cfg.Task.Type() != shared.ModelRunTaskName {
			continue
		}
		payload, err := shared.UnmarshalModelRunPayload(cfg.Task.Payload())
		if err != nil {
			t.Fatal(err)
		}
		key := store.ModelIdentityKey(payload.ProviderID, payload.ModelID)
		got[key] = cfg.Cronspec
		gotOffset[key] = processInDelay(cfg.Opts)
	}
	want := map[string]string{
		store.ModelIdentityKey("openai", "chat-a"):     "@every 15m0s",
		store.ModelIdentityKey("openai", "embed-a"):    "@every 30m0s",
		store.ModelIdentityKey("openai", "embed-fast"): "@every 2m0s",
	}
	for key, cronspec := range want {
		if got[key] != cronspec {
			t.Fatalf("model %q cronspec = %q, want %q (all: %#v)", key, got[key], cronspec, got)
		}
	}
	wantOffsets := map[string]time.Duration{
		store.ModelIdentityKey("openai", "chat-a"):     0,
		store.ModelIdentityKey("openai", "embed-a"):    shared.ModelRunSpacing,
		store.ModelIdentityKey("openai", "embed-fast"): 2 * shared.ModelRunSpacing,
	}
	for key, offset := range wantOffsets {
		if gotOffset[key] != offset {
			t.Fatalf("model %q offset = %s, want %s (all: %#v)", key, gotOffset[key], offset, gotOffset)
		}
	}
}

func TestScheduledModelRunPayloadIsStable(t *testing.T) {
	model := store.RunnableModel{ProviderID: "openai", ModelID: "chat-a", Capability: "chat"}
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
	embedDelay := 2 * shared.ModelRunSpacing
	chatPayload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{ProviderID: "openai", ModelID: "chat-a", Capability: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	embedPayload, err := shared.MarshalModelRunPayload(shared.ModelRunPayload{ProviderID: "openai", ModelID: "embed-a", Capability: "embedding"})
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
		{Task: asynq.NewTask(shared.ModelRunTaskName, embedPayload), Opts: []asynq.Option{asynq.ProcessIn(embedDelay)}, Next: embedNext},
	})

	if len(got) != 2 {
		t.Fatalf("next checks len = %d, want 2: %#v", len(got), got)
	}
	chatKey := store.ModelIdentityKey("openai", "chat-a")
	embedKey := store.ModelIdentityKey("openai", "embed-a")
	if !got[chatKey].Equal(chatNext) {
		t.Fatalf("chat next = %s, want %s", got[chatKey], chatNext)
	}
	if !got[embedKey].Equal(embedNext.Add(embedDelay)) {
		t.Fatalf("embed next = %s, want %s", got[embedKey], embedNext.Add(embedDelay))
	}
}
