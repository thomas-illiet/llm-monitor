package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// MissingModelsForAlert lists models missing longer than the alert threshold.
func (s *Store) MissingModelsForAlert(ctx context.Context, threshold time.Duration, now time.Time) ([]ModelState, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT model_id, capability, excluded, status, first_seen_at, last_seen_at, missing_since, skip_reason, last_probe_at
		FROM model_states
		WHERE status='missing' AND missing_since IS NOT NULL AND missing_since <= $1
		ORDER BY missing_since ASC
	`, now.Add(-threshold))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var models []ModelState
	for rows.Next() {
		var model ModelState
		var missing pgtype.Timestamptz
		var lastProbe pgtype.Timestamptz
		if err := rows.Scan(&model.ModelID, &model.Capability, &model.Excluded, &model.Status, &model.FirstSeenAt, &model.LastSeenAt, &missing, &model.SkipReason, &lastProbe); err != nil {
			return nil, err
		}
		if missing.Valid {
			model.MissingSince = &missing.Time
		}
		if lastProbe.Valid {
			model.LastProbeAt = &lastProbe.Time
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

// RecordHTTPCheck persists one LLM service HTTP availability check.
func (s *Store) RecordHTTPCheck(ctx context.Context, record CheckRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO http_checks(checked_at, ok, status_code, latency_ms, error)
		VALUES($1, $2, $3, $4, $5)
	`, record.At, record.OK, record.StatusCode, record.LatencyMS, record.Error)
	return err
}

// RecordAuthCheck persists one OAuth/token endpoint availability check.
func (s *Store) RecordAuthCheck(ctx context.Context, record CheckRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_checks(checked_at, ok, status_code, latency_ms, expires_at, error)
		VALUES($1, $2, $3, $4, $5, $6)
	`, record.At, record.OK, record.StatusCode, record.LatencyMS, record.ExpiresAt, record.Error)
	return err
}

// LatestHTTPCheck returns the newest HTTP check, if any.
func (s *Store) LatestHTTPCheck(ctx context.Context) (*CheckRecord, error) {
	return s.latestCheck(ctx, `SELECT checked_at, ok, status_code, latency_ms, NULL::timestamptz, error FROM http_checks ORDER BY checked_at DESC LIMIT 1`)
}

// LatestAuthCheck returns the newest auth check, if any.
func (s *Store) LatestAuthCheck(ctx context.Context) (*CheckRecord, error) {
	return s.latestCheck(ctx, `SELECT checked_at, ok, status_code, latency_ms, expires_at, error FROM auth_checks ORDER BY checked_at DESC LIMIT 1`)
}

// latestCheck scans a single latest-check query into a shared DTO.
func (s *Store) latestCheck(ctx context.Context, query string) (*CheckRecord, error) {
	var record CheckRecord
	var expires pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, query).Scan(&record.At, &record.OK, &record.StatusCode, &record.LatencyMS, &expires, &record.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expires.Valid {
		record.ExpiresAt = &expires.Time
	}
	return &record, nil
}
