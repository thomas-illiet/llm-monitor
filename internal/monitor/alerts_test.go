package monitor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/store"
)

type alertRepository struct {
	alerts map[string]store.EmailAlertRecord
	events []store.ModelEventRecord
}

func (r *alertRepository) RecordHTTPCheck(context.Context, store.CheckRecord) error {
	return nil
}

func (r *alertRepository) RecordAuthCheck(context.Context, store.CheckRecord) error {
	return nil
}

func (r *alertRepository) ProcessModelObservation(context.Context, []store.ObservedModel, time.Time) ([]store.ModelEvent, error) {
	return nil, nil
}

func (r *alertRepository) LastRunnableCapabilities(context.Context) (map[string]string, error) {
	return nil, nil
}

func (r *alertRepository) MissingModelsForAlert(context.Context, time.Duration, time.Time) ([]store.ModelState, error) {
	return nil, nil
}

func (r *alertRepository) EmailAlertExists(_ context.Context, key string) (bool, error) {
	record, ok := r.alerts[key]
	return ok && record.Error == "", nil
}

func (r *alertRepository) RecordEmailAlert(_ context.Context, record store.EmailAlertRecord) error {
	if r.alerts == nil {
		r.alerts = map[string]store.EmailAlertRecord{}
	}
	existing, ok := r.alerts[record.AlertKey]
	if ok && existing.Error == "" {
		return nil
	}
	r.alerts[record.AlertKey] = record
	return nil
}

func (r *alertRepository) RecordChatRun(context.Context, store.ChatRunRecord) error {
	return nil
}

func (r *alertRepository) RecordEmbeddingRun(context.Context, store.EmbeddingRunRecord) error {
	return nil
}

func (r *alertRepository) RecordModelEvent(_ context.Context, record store.ModelEventRecord) error {
	r.events = append(r.events, record)
	return nil
}

func (r *alertRepository) PruneHistoryBefore(context.Context, time.Time) error {
	return nil
}

type scriptedNotifier struct {
	calls int
	errs  []error
}

func (n *scriptedNotifier) Send(notify.Message) error {
	n.calls++
	if len(n.errs) == 0 {
		return nil
	}
	err := n.errs[0]
	n.errs = n.errs[1:]
	return err
}

// TestSendModelAlertRetriesAfterFailureAndDedupesSuccess verifies failed attempts do not suppress later successful alerts.
func TestSendModelAlertRetriesAfterFailureAndDedupesSuccess(t *testing.T) {
	repo := &alertRepository{alerts: map[string]store.EmailAlertRecord{}}
	notifier := &scriptedNotifier{errs: []error{errors.New("smtp unavailable"), nil}}
	scheduler := NewScheduler(config.Config{
		Dashboard: config.DashboardConfig{SiteName: "LLM Monitor", SiteURL: "https://monitor.example.test"},
		SMTP:      config.SMTPConfig{To: []string{"platform@example.test"}},
	}, repo, nil, nil, notifier, slog.New(slog.NewTextHandler(io.Discard, nil)))
	key := modelAlertKey("missing", "gpt-test", time.Unix(42, 0).UTC())

	scheduler.sendModelAlert(context.Background(), key, "gpt-test", "missing", "Model missing", "Model gpt-test is missing.", nil)
	if notifier.calls != 1 {
		t.Fatalf("send calls after failure = %d, want 1", notifier.calls)
	}
	if repo.alerts[key].Error == "" {
		t.Fatalf("first alert record error = %q, want failure preserved", repo.alerts[key].Error)
	}

	scheduler.sendModelAlert(context.Background(), key, "gpt-test", "missing", "Model missing", "Model gpt-test is missing.", nil)
	if notifier.calls != 2 {
		t.Fatalf("send calls after retry = %d, want 2", notifier.calls)
	}
	if repo.alerts[key].Error != "" {
		t.Fatalf("retry alert record error = %q, want successful delivery", repo.alerts[key].Error)
	}

	scheduler.sendModelAlert(context.Background(), key, "gpt-test", "missing", "Model missing", "Model gpt-test is missing.", nil)
	if notifier.calls != 2 {
		t.Fatalf("send calls after successful dedupe = %d, want 2", notifier.calls)
	}
}
