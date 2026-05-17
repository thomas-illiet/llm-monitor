package store

import (
	"context"
	"time"
)

// RecordChatRun stores performance metrics from one chat completion probe.
func (s *Store) RecordChatRun(ctx context.Context, record ChatRunRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_runs(
			model_id, prompt_id, started_at, ok, status_code, latency_ms, ttft_ms, itl_ms, tpot_ms,
			request_latency_ms, input_tokens, output_tokens, total_tokens, output_tokens_per_second, error
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, record.ModelID, record.PromptID, record.StartedAt, record.OK, record.StatusCode, record.LatencyMS, record.TTFTMS, record.ITLMS, record.TPOTMS, record.RequestLatencyMS, record.InputTokens, record.OutputTokens, record.TotalTokens, record.OutputTokensPerSecond, record.Error)
	return err
}

// RecordEmbeddingRun stores performance metrics from one embedding probe.
func (s *Store) RecordEmbeddingRun(ctx context.Context, record EmbeddingRunRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO embedding_runs(model_id, fixture_path, fixture_bytes, started_at, ok, status_code, latency_ms, input_tokens, total_tokens, vector_dimensions, error)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, record.ModelID, record.FixturePath, record.FixtureBytes, record.StartedAt, record.OK, record.StatusCode, record.LatencyMS, record.InputTokens, record.TotalTokens, record.VectorDimensions, record.Error)
	return err
}

// RecentRuns returns recent chat and embedding probes in one timeline.
func (s *Store) RecentRuns(ctx context.Context, limit int) ([]RecentRun, error) {
	return s.recentRuns(ctx, "", nil, limit)
}

// RecentRunsForModel returns recent chat and embedding probes for one model.
func (s *Store) RecentRunsForModel(ctx context.Context, modelID string, since time.Time, limit int) ([]RecentRun, error) {
	return s.recentRuns(ctx, modelID, &since, limit)
}

// recentRuns returns recent probes, optionally scoped to a model ID.
func (s *Store) recentRuns(ctx context.Context, modelID string, since *time.Time, limit int) ([]RecentRun, error) {
	var sinceArg any
	if since != nil {
		sinceArg = *since
	}
	rows, err := s.pool.Query(ctx, `
		SELECT kind, model_id, prompt_id, started_at, ok, status_code, latency_ms, input_tokens, output_tokens, total_tokens, error
		FROM (
			SELECT 'chat' AS kind, model_id, prompt_id, started_at, ok, status_code, latency_ms, input_tokens, output_tokens, total_tokens, error
			FROM chat_runs
			UNION ALL
			SELECT 'embedding' AS kind, model_id, '' AS prompt_id, started_at, ok, status_code, latency_ms, input_tokens, NULL::integer AS output_tokens, total_tokens, error
			FROM embedding_runs
		) runs
		WHERE ($2 = '' OR model_id = $2)
			AND ($3::timestamptz IS NULL OR started_at >= $3)
		ORDER BY started_at DESC
		LIMIT $1
	`, limit, modelID, sinceArg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []RecentRun
	for rows.Next() {
		var run RecentRun
		if err := rows.Scan(&run.Kind, &run.ModelID, &run.PromptID, &run.StartedAt, &run.OK, &run.StatusCode, &run.LatencyMS, &run.InputTokens, &run.OutputTokens, &run.TotalTokens, &run.Error); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
