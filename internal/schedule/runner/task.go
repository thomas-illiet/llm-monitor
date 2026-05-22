package runner

import (
	"context"
	"encoding/json"
	"time"
)

// Handler executes one task invocation.
type Handler func(context.Context, TaskContext) error

// Task describes a named business task independently from its execution mode.
type Task struct {
	Name        string
	Handler     Handler
	Timeout     time.Duration
	MaxAttempts int
}

// TaskContext contains serializable execution metadata for local and future distributed runners.
type TaskContext struct {
	TaskName    string          `json:"task_name"`
	RunID       string          `json:"run_id"`
	Attempt     int             `json:"attempt"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}
