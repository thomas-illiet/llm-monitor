package runner

import (
	"context"
	"testing"
)

func TestRegistryRejectsDuplicateTaskNames(t *testing.T) {
	registry := NewRegistry()
	task := Task{Name: "monitor.example", Handler: func(context.Context, TaskContext) error { return nil }}

	if err := registry.Register(task); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(task); err == nil {
		t.Fatal("expected duplicate registration to fail")
	}
}
