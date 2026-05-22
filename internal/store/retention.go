package store

import (
	"context"
	"time"
)

type historyPruneStatement struct {
	query string
	args  []any
}

// PruneHistoryBefore deletes persisted history older than cutoff while preserving current state.
func (s *Store) PruneHistoryBefore(ctx context.Context, cutoff time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, statement := range historyPruneStatements(cutoff) {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// historyPruneStatements returns the deletion queries used by retention pruning.
func historyPruneStatements(cutoff time.Time) []historyPruneStatement {
	return []historyPruneStatement{
		{query: `DELETE FROM model_events WHERE observed_at < $1`, args: []any{cutoff}},
		{query: `DELETE FROM http_checks WHERE checked_at < $1`, args: []any{cutoff}},
		{query: `DELETE FROM auth_checks WHERE checked_at < $1`, args: []any{cutoff}},
		{query: `DELETE FROM chat_runs WHERE started_at < $1`, args: []any{cutoff}},
		{query: `DELETE FROM embedding_runs WHERE started_at < $1`, args: []any{cutoff}},
		{
			query: `
				DELETE FROM email_alerts alerts
				WHERE alerts.sent_at < $1
					AND NOT (
						alerts.alert_type IN ('inactive', 'missing')
						AND EXISTS (
							SELECT 1
							FROM model_states states
							WHERE states.provider_id = alerts.provider_id
								AND states.model_id = alerts.model_id
								AND states.status = 'inactive'
								AND states.missing_since IS NOT NULL
						)
					)
			`,
			args: []any{cutoff},
		},
		{query: `DELETE FROM model_snapshots WHERE observed_at < $1`, args: []any{cutoff}},
	}
}
