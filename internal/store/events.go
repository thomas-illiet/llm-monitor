package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	modelEventSelectColumns = `id, provider_id, model_id, event_type, source, severity, status, capability, observed_at, title, message, changed, details`
	insertModelEventSQL     = `
		INSERT INTO model_events(provider_id, model_id, event_type, source, severity, status, capability, observed_at, title, message, changed, details)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, provider_id, model_id, event_type, capability, observed_at, changed
	`
	recordModelEventSQL = `
		INSERT INTO model_events(provider_id, model_id, event_type, source, severity, status, capability, observed_at, title, message, changed, details)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	recentModelEventsSQL = `
		SELECT ` + modelEventSelectColumns + `
		FROM model_events
		WHERE changed
		ORDER BY observed_at DESC
		LIMIT $1
	`
)

// insertModelEvent appends one lifecycle event and returns alert-facing row data.
func insertModelEvent(ctx context.Context, tx pgx.Tx, record ModelEventRecord) (ModelEvent, error) {
	normalizeModelEvent(&record)
	raw, err := json.Marshal(record.Details)
	if err != nil {
		return ModelEvent{}, err
	}
	var event ModelEvent
	err = tx.QueryRow(ctx, insertModelEventSQL, record.ProviderID, record.ModelID, record.EventType, record.Source, record.Severity, record.Status, record.Capability, record.ObservedAt, record.Title, record.Message, record.Changed, raw).Scan(&event.ID, &event.ProviderID, &event.ModelID, &event.EventType, &event.Capability, &event.ObservedAt, &event.Changed)
	return event, err
}

// RecordModelEvent writes a model-scoped diagnostic event.
func (s *Store) RecordModelEvent(ctx context.Context, record ModelEventRecord) error {
	normalizeModelEvent(&record)
	raw, err := json.Marshal(record.Details)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, recordModelEventSQL, record.ProviderID, record.ModelID, record.EventType, record.Source, record.Severity, record.Status, record.Capability, record.ObservedAt, record.Title, record.Message, record.Changed, raw)
	return err
}

// normalizeModelEvent fills defaults expected by model_events writes.
func normalizeModelEvent(record *ModelEventRecord) {
	if record.ObservedAt.IsZero() {
		record.ObservedAt = time.Now().UTC()
	}
	if record.Source == "" {
		record.Source = "monitor"
	}
	if record.Severity == "" {
		record.Severity = "info"
	}
	if record.Status == "" {
		record.Status = "ok"
	}
	if record.Capability == "" {
		record.Capability = "unknown"
	}
	if record.Title == "" {
		record.Title = record.EventType
	}
	if record.Message == "" {
		record.Message = record.Title
	}
	if record.Details == nil {
		record.Details = map[string]any{}
	}
}

// eventSeverity maps a model capability result to timeline severity.
func eventSeverity(capability, skipReason string) string {
	if capability == "unknown" && skipReason != "" {
		return "error"
	}
	if capability == "skip" || skipReason != "" {
		return "warning"
	}
	return "info"
}

// eventStatus maps a model capability result to timeline status.
func eventStatus(capability, skipReason string) string {
	if capability == "unknown" && skipReason != "" {
		return "error"
	}
	if capability == "skip" || skipReason != "" {
		return "skipped"
	}
	return "ok"
}

// RecentModelEvents returns the newest model events for the dashboard.
func (s *Store) RecentModelEvents(ctx context.Context, limit int) ([]RecentEvent, error) {
	rows, err := s.pool.Query(ctx, recentModelEventsSQL, limit)
	if err != nil {
		return nil, err
	}
	return scanRecentEvents(rows)
}

// ListModelEvents returns a filtered, model-scoped diagnostic timeline page.
func (s *Store) ListModelEvents(ctx context.Context, query ModelEventQuery) (ModelEventPage, error) {
	var page ModelEventPage
	where, args := modelEventWhereClause(query)

	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM model_events `+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}

	limitParam := len(args) + 1
	offsetParam := len(args) + 2
	args = append(args, query.Limit, query.Offset)
	rows, err := s.pool.Query(ctx, `
		SELECT `+modelEventSelectColumns+`
		FROM model_events
		`+where+`
		ORDER BY observed_at DESC
		LIMIT $`+fmt.Sprint(limitParam)+` OFFSET $`+fmt.Sprint(offsetParam), args...)
	if err != nil {
		return page, err
	}
	page.Events, err = scanRecentEvents(rows)
	if err != nil {
		return page, err
	}
	page.Filters, err = s.ModelEventFilterOptions(ctx, query.ProviderID, query.ModelID)
	return page, err
}

// modelEventWhereClause builds the parameterized WHERE clause for event filters.
func modelEventWhereClause(query ModelEventQuery) (string, []any) {
	args := []any{query.ProviderID, query.ModelID}
	clauses := []string{"provider_id=$1", "model_id=$2"}
	if len(query.Statuses) > 0 {
		args = append(args, query.Statuses)
		clauses = append(clauses, fmt.Sprintf("status = ANY($%d)", len(args)))
	}
	if len(query.Sources) > 0 {
		args = append(args, query.Sources)
		clauses = append(clauses, fmt.Sprintf("source = ANY($%d)", len(args)))
	}
	if len(query.EventTypes) > 0 {
		args = append(args, query.EventTypes)
		clauses = append(clauses, fmt.Sprintf("event_type = ANY($%d)", len(args)))
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// ModelEventFilterOptions returns all available event filters for one model.
func (s *Store) ModelEventFilterOptions(ctx context.Context, providerID, modelID string) (ModelEventFilterOptions, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT category, value
		FROM (
			SELECT 'status' AS category, status AS value
			FROM model_events
			WHERE provider_id=$1 AND model_id=$2 AND status <> ''
			GROUP BY status
			UNION ALL
			SELECT 'source' AS category, source AS value
			FROM model_events
			WHERE provider_id=$1 AND model_id=$2 AND source <> ''
			GROUP BY source
			UNION ALL
			SELECT 'event_type' AS category, event_type AS value
			FROM model_events
			WHERE provider_id=$1 AND model_id=$2 AND event_type <> ''
			GROUP BY event_type
		) filters
		ORDER BY category, value
	`, providerID, modelID)
	if err != nil {
		return ModelEventFilterOptions{}, err
	}
	defer rows.Close()

	options := ModelEventFilterOptions{
		Statuses:   []string{},
		Sources:    []string{},
		EventTypes: []string{},
	}
	for rows.Next() {
		var category, value string
		if err := rows.Scan(&category, &value); err != nil {
			return ModelEventFilterOptions{}, err
		}
		switch category {
		case "status":
			options.Statuses = append(options.Statuses, value)
		case "source":
			options.Sources = append(options.Sources, value)
		case "event_type":
			options.EventTypes = append(options.EventTypes, value)
		}
	}
	return options, rows.Err()
}

// scanRecentEvents converts model event rows into dashboard event DTOs.
func scanRecentEvents(rows pgx.Rows) ([]RecentEvent, error) {
	defer rows.Close()
	var events []RecentEvent
	for rows.Next() {
		var event RecentEvent
		var raw []byte
		if err := rows.Scan(&event.ID, &event.ProviderID, &event.ModelID, &event.EventType, &event.Source, &event.Severity, &event.Status, &event.Capability, &event.ObservedAt, &event.Title, &event.Message, &event.Changed, &raw); err != nil {
			return nil, err
		}
		event.ModelKey = ModelKey(event.ModelID)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &event.Details); err != nil {
				return nil, err
			}
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
