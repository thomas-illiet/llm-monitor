package models

import (
	"context"
	"time"

	"llmservicemonitor/internal/store"
)

type noopRepository struct{}

func (r noopRepository) RecordHTTPCheck(context.Context, store.CheckRecord) error {
	return nil
}

func (r noopRepository) RecordAuthCheck(context.Context, store.CheckRecord) error {
	return nil
}

func (r noopRepository) ProcessModelObservation(context.Context, []store.ObservedModel, time.Time) ([]store.ModelEvent, error) {
	return nil, nil
}

func (r noopRepository) MarkModelInactive(context.Context, string, time.Time, string, string) (*store.ModelEvent, error) {
	return nil, nil
}

func (r noopRepository) MarkAllModelsInactive(context.Context, time.Time, string, string) ([]store.ModelEvent, error) {
	return nil, nil
}

func (r noopRepository) LastRunnableCapabilities(context.Context) (map[string]string, error) {
	return nil, nil
}

func (r noopRepository) InactiveModelsForAlert(context.Context, time.Duration, time.Time) ([]store.ModelState, error) {
	return nil, nil
}

func (r noopRepository) EmailAlertExists(context.Context, string) (bool, error) {
	return false, nil
}

func (r noopRepository) RecordEmailAlert(context.Context, store.EmailAlertRecord) error {
	return nil
}

func (r noopRepository) RecordChatRun(context.Context, store.ChatRunRecord) error {
	return nil
}

func (r noopRepository) RecordEmbeddingRun(context.Context, store.EmbeddingRunRecord) error {
	return nil
}

func (r noopRepository) RecordModelEvent(context.Context, store.ModelEventRecord) error {
	return nil
}

func (r noopRepository) PruneHistoryBefore(context.Context, time.Time) error {
	return nil
}
