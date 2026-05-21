package tasks

import (
	"slices"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
)

func TestNewRegistryRegistersStableTaskNames(t *testing.T) {
	registry, err := NewRegistry(Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	got := registry.Names()
	want := []string{
		AuthCheckTaskName,
		HistoryRetentionTaskName,
		HTTPCheckTaskName,
		ModelRunTaskName,
		ModelSnapshotTaskName,
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("registered names = %v, want %v", got, want)
	}
}

func TestLocalScheduleGroupsPreserveModelStartupOrder(t *testing.T) {
	groups := LocalScheduleGroups(Dependencies{Config: config.Config{
		Schedules: config.ScheduleConfig{
			HTTPCheck:     config.Duration{Duration: 30 * time.Second},
			AuthCheck:     config.Duration{Duration: time.Minute},
			ModelSnapshot: config.Duration{Duration: 5 * time.Minute},
			ModelRuns:     config.Duration{Duration: 15 * time.Minute},
		},
		Retention: config.RetentionConfig{History: config.Duration{Duration: 90 * 24 * time.Hour}},
	}})

	var modelGroupFound bool
	for _, group := range groups {
		if group.Name != "models" {
			continue
		}
		modelGroupFound = true
		if len(group.Startup) != 1 || group.Startup[0].TaskName != ModelSnapshotTaskName {
			t.Fatalf("model startup = %+v, want initial model snapshot", group.Startup)
		}
		if len(group.Recurring) != 1 {
			t.Fatalf("model recurring schedules = %+v, want snapshot only", group.Recurring)
		}
		if group.Recurring[0].TaskName != ModelSnapshotTaskName || group.Recurring[0].RunImmediately {
			t.Fatalf("first recurring model schedule = %+v, want delayed snapshot", group.Recurring[0])
		}
	}
	if !modelGroupFound {
		t.Fatal("models schedule group not found")
	}
}
