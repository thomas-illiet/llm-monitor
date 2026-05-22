package queue

import (
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/config"
)

func TestTaskOptionsApplySixtySecondTimeout(t *testing.T) {
	opts := taskOptions(config.Config{Asynq: config.AsynqConfig{Queue: "monitor"}})

	if got := optionValue[time.Duration](t, opts, asynq.TimeoutOpt); got != taskTimeout {
		t.Fatalf("timeout = %s, want %s", got, taskTimeout)
	}
	if got := optionValue[int](t, opts, asynq.MaxRetryOpt); got != 0 {
		t.Fatalf("max retry = %d, want 0", got)
	}
}

func TestManualTaskOptionsExpireIfNotStartedWithinTimeout(t *testing.T) {
	retention := 10 * time.Minute
	before := time.Now().UTC()
	opts := manualTaskOptions(config.Config{
		Asynq: config.AsynqConfig{
			Queue:               "monitor",
			ManualTaskRetention: config.Duration{Duration: retention},
		},
	})
	after := time.Now().UTC()

	deadline := optionValue[time.Time](t, opts, asynq.DeadlineOpt)
	if deadline.Before(before.Add(taskTimeout)) || deadline.After(after.Add(taskTimeout)) {
		t.Fatalf("deadline = %s, want between %s and %s", deadline, before.Add(taskTimeout), after.Add(taskTimeout))
	}
	if got := optionValue[time.Duration](t, opts, asynq.RetentionOpt); got != retention {
		t.Fatalf("retention = %s, want %s", got, retention)
	}
}

func optionValue[T any](t *testing.T, opts []asynq.Option, optionType asynq.OptionType) T {
	t.Helper()
	for _, opt := range opts {
		if opt.Type() != optionType {
			continue
		}
		value, ok := opt.Value().(T)
		if !ok {
			t.Fatalf("option %v has value %T, want requested type", optionType, opt.Value())
		}
		return value
	}
	var zero T
	t.Fatalf("option %v not found", optionType)
	return zero
}
