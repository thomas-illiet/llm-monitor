package tasks

import (
	"slices"
	"testing"
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
