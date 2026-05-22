package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ReserveTaskStart atomically reserves the earliest allowed start time for a shared task key.
func (s *Store) ReserveTaskStart(ctx context.Context, key string, earliest time.Time, spacing time.Duration) (time.Time, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return time.Time{}, fmt.Errorf("task spacing key is required")
	}
	if spacing <= 0 {
		return time.Time{}, fmt.Errorf("task spacing must be greater than 0")
	}
	if earliest.IsZero() {
		earliest = time.Now().UTC()
	} else {
		earliest = earliest.UTC()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO task_spacing_state(key, next_allowed_at)
		VALUES ($1, $2)
		ON CONFLICT (key) DO NOTHING
	`, key, earliest); err != nil {
		return time.Time{}, err
	}

	var nextAllowed time.Time
	if err := tx.QueryRow(ctx, `
		SELECT next_allowed_at
		FROM task_spacing_state
		WHERE key=$1
		FOR UPDATE
	`, key).Scan(&nextAllowed); err != nil {
		return time.Time{}, err
	}

	reservedAt := earliest
	if nextAllowed.After(reservedAt) {
		reservedAt = nextAllowed.UTC()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE task_spacing_state
		SET next_allowed_at=$2
		WHERE key=$1
	`, key, reservedAt.Add(spacing)); err != nil {
		return time.Time{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}
	return reservedAt, nil
}
