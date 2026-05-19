package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ProcessModelObservation stores a snapshot and derives model lifecycle events.
func (s *Store) ProcessModelObservation(ctx context.Context, observed []ObservedModel, now time.Time) ([]ModelEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	raw, err := json.Marshal(observed)
	if err != nil {
		return nil, err
	}
	var snapshotID int64
	if err := tx.QueryRow(ctx, `INSERT INTO model_snapshots(observed_at, raw) VALUES($1, $2) RETURNING id`, now, raw).Scan(&snapshotID); err != nil {
		return nil, err
	}
	for _, model := range observed {
		probeDetails, err := json.Marshal(model.ProbeDetails)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO model_snapshot_items(snapshot_id, model_id, capability, excluded, skip_reason, probe_details)
			VALUES($1, $2, $3, $4, $5, $6)
		`, snapshotID, model.ID, model.Capability, model.Excluded, model.SkipReason, probeDetails); err != nil {
			return nil, err
		}
	}

	current, err := loadModelStates(ctx, tx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]ObservedModel, len(observed))
	events := make([]ModelEvent, 0)
	for _, model := range observed {
		seen[model.ID] = model
		state, exists := current[model.ID]
		switch {
		case !exists:
			if _, err := tx.Exec(ctx, `
				INSERT INTO model_states(model_id, capability, excluded, status, first_seen_at, last_seen_at, missing_since, skip_reason, last_probe_at)
				VALUES($1, $2, $3, $4, $5, $5, NULL, $6, $5)
			`, model.ID, model.Capability, model.Excluded, ModelStatusActive, now, model.SkipReason); err != nil {
				return nil, err
			}
			event, err := insertModelEvent(ctx, tx, ModelEventRecord{
				ModelID:    model.ID,
				EventType:  "added",
				Source:     "inventory",
				Severity:   eventSeverity(model.Capability, model.SkipReason),
				Status:     eventStatus(model.Capability, model.SkipReason),
				Capability: model.Capability,
				ObservedAt: now,
				Title:      "Model discovered",
				Message:    fmt.Sprintf("Model %s was discovered with capability %s.", model.ID, model.Capability),
				Changed:    true,
				Details: map[string]any{
					"first_seen":    true,
					"skip_reason":   model.SkipReason,
					"probe_details": model.ProbeDetails,
				},
			})
			if err != nil {
				return nil, err
			}
			event.FirstSeen = true
			events = append(events, event)
		case state.MissingSince != nil:
			missingDuration := now.Sub(*state.MissingSince)
			if _, err := tx.Exec(ctx, `
				UPDATE model_states
				SET capability=$2, excluded=$3, status=$4, last_seen_at=$5, missing_since=NULL, skip_reason=$6, last_probe_at=$5
				WHERE model_id=$1
			`, model.ID, model.Capability, model.Excluded, ModelStatusActive, now, model.SkipReason); err != nil {
				return nil, err
			}
			event, err := insertModelEvent(ctx, tx, ModelEventRecord{
				ModelID:    model.ID,
				EventType:  "returned",
				Source:     "inventory",
				Severity:   eventSeverity(model.Capability, model.SkipReason),
				Status:     eventStatus(model.Capability, model.SkipReason),
				Capability: model.Capability,
				ObservedAt: now,
				Title:      "Model returned",
				Message:    fmt.Sprintf("Model %s returned after being absent for %s.", model.ID, missingDuration.Round(time.Second)),
				Changed:    true,
				Details: map[string]any{
					"missing_since": state.MissingSince.Format(time.RFC3339),
					"absent_ms":     missingDuration.Milliseconds(),
					"skip_reason":   model.SkipReason,
					"probe_details": model.ProbeDetails,
				},
			})
			if err != nil {
				return nil, err
			}
			event.MissingSince = state.MissingSince
			event.MissingDuration = missingDuration
			events = append(events, event)
		default:
			if _, err := tx.Exec(ctx, `
				UPDATE model_states SET capability=$2, excluded=$3, status=$4, last_seen_at=$5, skip_reason=$6, last_probe_at=$5
				WHERE model_id=$1
			`, model.ID, model.Capability, model.Excluded, ModelStatusActive, now, model.SkipReason); err != nil {
				return nil, err
			}
			if state.Capability != model.Capability || state.Excluded != model.Excluded || state.SkipReason != model.SkipReason {
				event, err := insertModelEvent(ctx, tx, ModelEventRecord{
					ModelID:    model.ID,
					EventType:  "capability_changed",
					Source:     "inventory",
					Severity:   eventSeverity(model.Capability, model.SkipReason),
					Status:     eventStatus(model.Capability, model.SkipReason),
					Capability: model.Capability,
					ObservedAt: now,
					Title:      "Model capability changed",
					Message:    fmt.Sprintf("Model %s changed from %s to %s.", model.ID, state.Capability, model.Capability),
					Changed:    true,
					Details: map[string]any{
						"previous_capability":  state.Capability,
						"current_capability":   model.Capability,
						"previous_excluded":    state.Excluded,
						"current_excluded":     model.Excluded,
						"previous_skip_reason": state.SkipReason,
						"current_skip_reason":  model.SkipReason,
						"probe_details":        model.ProbeDetails,
					},
				})
				if err != nil {
					return nil, err
				}
				events = append(events, event)
			}
		}
	}

	for modelID, state := range current {
		if _, ok := seen[modelID]; ok || state.MissingSince != nil || state.Status == ModelStatusInactive {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE model_states SET status=$2, missing_since=$3 WHERE model_id=$1
		`, modelID, ModelStatusInactive, now); err != nil {
			return nil, err
		}
		event, err := insertModelEvent(ctx, tx, ModelEventRecord{
			ModelID:    modelID,
			EventType:  "inactive",
			Source:     "inventory",
			Severity:   "warning",
			Status:     ModelStatusInactive,
			Capability: state.Capability,
			ObservedAt: now,
			Title:      "Model inactive",
			Message:    fmt.Sprintf("Model %s disappeared from /v1/models.", modelID),
			Changed:    true,
			Details: map[string]any{
				"inactive_since": now.Format(time.RFC3339),
				"missing_since":  now.Format(time.RFC3339),
			},
		})
		if err != nil {
			return nil, err
		}
		event.MissingSince = &now
		events = append(events, event)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}

// MarkModelInactive transitions one runnable model to inactive when a probe proves it unavailable.
func (s *Store) MarkModelInactive(ctx context.Context, modelID string, now time.Time, source, reason string) (*ModelEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	state, err := loadModelStateForUpdate(ctx, tx, modelID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if state.Status == ModelStatusInactive || state.Excluded || state.Capability == "skip" {
		return nil, tx.Commit(ctx)
	}
	event, err := markModelStateInactive(ctx, tx, state, now, source, reason)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &event, nil
}

// MarkAllModelsInactive transitions all currently runnable models to inactive.
func (s *Store) MarkAllModelsInactive(ctx context.Context, now time.Time, source, reason string) ([]ModelEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT model_id, capability, excluded, status, first_seen_at, last_seen_at, missing_since, skip_reason, last_probe_at
		FROM model_states
		WHERE status <> $1 AND NOT excluded AND capability <> 'skip'
		FOR UPDATE
	`, ModelStatusInactive)
	if err != nil {
		return nil, err
	}
	states, err := scanModelStates(rows)
	if err != nil {
		return nil, err
	}
	events := make([]ModelEvent, 0, len(states))
	for _, state := range states {
		event, err := markModelStateInactive(ctx, tx, state, now, source, reason)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}

func loadModelStateForUpdate(ctx context.Context, tx pgx.Tx, modelID string) (ModelState, error) {
	rows, err := tx.Query(ctx, `
		SELECT model_id, capability, excluded, status, first_seen_at, last_seen_at, missing_since, skip_reason, last_probe_at
		FROM model_states
		WHERE model_id=$1
		FOR UPDATE
	`, modelID)
	if err != nil {
		return ModelState{}, err
	}
	states, err := scanModelStates(rows)
	if err != nil {
		return ModelState{}, err
	}
	if len(states) == 0 {
		return ModelState{}, pgx.ErrNoRows
	}
	return states[0], nil
}

func markModelStateInactive(ctx context.Context, tx pgx.Tx, state ModelState, now time.Time, source, reason string) (ModelEvent, error) {
	if source == "" {
		source = "monitor"
	}
	if reason == "" {
		reason = "model is unavailable"
	}
	inactiveSince := now
	if state.MissingSince != nil {
		inactiveSince = *state.MissingSince
	}
	if _, err := tx.Exec(ctx, `
		UPDATE model_states
		SET status=$2, missing_since=COALESCE(missing_since, $3), skip_reason=$4, last_probe_at=$3
		WHERE model_id=$1
	`, state.ModelID, ModelStatusInactive, now, reason); err != nil {
		return ModelEvent{}, err
	}
	event, err := insertModelEvent(ctx, tx, ModelEventRecord{
		ModelID:    state.ModelID,
		EventType:  "inactive",
		Source:     source,
		Severity:   "error",
		Status:     ModelStatusInactive,
		Capability: state.Capability,
		ObservedAt: now,
		Title:      "Model inactive",
		Message:    fmt.Sprintf("Model %s was marked inactive: %s.", state.ModelID, reason),
		Changed:    true,
		Details: map[string]any{
			"inactive_since": inactiveSince.Format(time.RFC3339),
			"missing_since":  inactiveSince.Format(time.RFC3339),
			"reason":         reason,
		},
	})
	if err != nil {
		return ModelEvent{}, err
	}
	event.MissingSince = &inactiveSince
	event.MissingDuration = now.Sub(inactiveSince)
	return event, nil
}

// loadModelStates reads current model state inside an observation transaction.
func loadModelStates(ctx context.Context, tx pgx.Tx) (map[string]ModelState, error) {
	rows, err := tx.Query(ctx, `
		SELECT model_id, capability, excluded, status, first_seen_at, last_seen_at, missing_since, skip_reason, last_probe_at
		FROM model_states
	`)
	if err != nil {
		return nil, err
	}
	states, err := scanModelStates(rows)
	if err != nil {
		return nil, err
	}
	current := map[string]ModelState{}
	for _, state := range states {
		current[state.ModelID] = state
	}
	return current, nil
}

func scanModelStates(rows pgx.Rows) ([]ModelState, error) {
	defer rows.Close()
	var ordered []ModelState
	for rows.Next() {
		var state ModelState
		var missing pgtype.Timestamptz
		var lastProbe pgtype.Timestamptz
		if err := rows.Scan(&state.ModelID, &state.Capability, &state.Excluded, &state.Status, &state.FirstSeenAt, &state.LastSeenAt, &missing, &state.SkipReason, &lastProbe); err != nil {
			return nil, err
		}
		if missing.Valid {
			state.MissingSince = &missing.Time
		}
		if lastProbe.Valid {
			state.LastProbeAt = &lastProbe.Time
		}
		ordered = append(ordered, state)
	}
	return ordered, rows.Err()
}

// ListModelStates returns the current inventory shown in the dashboard.
func (s *Store) ListModelStates(ctx context.Context) ([]ModelState, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT model_id, capability, excluded, status, first_seen_at, last_seen_at, missing_since, skip_reason, last_probe_at
		FROM model_states
		ORDER BY status ASC, model_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var states []ModelState
	for rows.Next() {
		var state ModelState
		var missing pgtype.Timestamptz
		var lastProbe pgtype.Timestamptz
		if err := rows.Scan(&state.ModelID, &state.Capability, &state.Excluded, &state.Status, &state.FirstSeenAt, &state.LastSeenAt, &missing, &state.SkipReason, &lastProbe); err != nil {
			return nil, err
		}
		if missing.Valid {
			state.MissingSince = &missing.Time
		}
		if lastProbe.Valid {
			state.LastProbeAt = &lastProbe.Time
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

// LastRunnableCapabilities returns the newest known chat/embedding capability per model.
func (s *Store) LastRunnableCapabilities(ctx context.Context) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (model_id) model_id, capability
		FROM (
			SELECT model_id, capability, last_seen_at AS observed_at
			FROM model_states
			WHERE capability IN ('chat', 'embedding')
			UNION ALL
			SELECT item.model_id, item.capability, snapshot.observed_at
			FROM model_snapshot_items item
			JOIN model_snapshots snapshot ON snapshot.id = item.snapshot_id
			WHERE item.capability IN ('chat', 'embedding')
		) known
		ORDER BY model_id, observed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	capabilities := map[string]string{}
	for rows.Next() {
		var modelID string
		var capability string
		if err := rows.Scan(&modelID, &capability); err != nil {
			return nil, err
		}
		capabilities[modelID] = capability
	}
	return capabilities, rows.Err()
}
