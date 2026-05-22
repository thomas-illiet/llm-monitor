package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// RecordChatRun stores performance metrics from one chat completion probe.
func (s *Store) RecordChatRun(ctx context.Context, record ChatRunRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_runs(
			provider_id, model_id, prompt_id, started_at, ok, status_code, latency_ms, ttft_ms, itl_ms, tpot_ms,
			request_latency_ms, input_tokens, output_tokens, total_tokens, output_tokens_per_second, error
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, record.ProviderID, record.ModelID, record.PromptID, record.StartedAt, record.OK, record.StatusCode, record.LatencyMS, record.TTFTMS, record.ITLMS, record.TPOTMS, record.RequestLatencyMS, record.InputTokens, record.OutputTokens, record.TotalTokens, record.OutputTokensPerSecond, record.Error)
	return err
}

// RecordEmbeddingRun stores performance metrics from one embedding probe.
func (s *Store) RecordEmbeddingRun(ctx context.Context, record EmbeddingRunRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO embedding_runs(provider_id, model_id, fixture_path, fixture_bytes, started_at, ok, status_code, latency_ms, input_tokens, total_tokens, vector_dimensions, error)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, record.ProviderID, record.ModelID, record.FixturePath, record.FixtureBytes, record.StartedAt, record.OK, record.StatusCode, record.LatencyMS, record.InputTokens, record.TotalTokens, record.VectorDimensions, record.Error)
	return err
}

// RecentRuns returns recent chat and embedding probes in one timeline.
func (s *Store) RecentRuns(ctx context.Context, limit int) ([]RecentRun, error) {
	return s.recentRuns(ctx, "", "", nil, limit)
}

// RecentRunsForModel returns recent chat and embedding probes for one model.
func (s *Store) RecentRunsForModel(ctx context.Context, providerID, modelID string, since time.Time, limit int) ([]RecentRun, error) {
	return s.recentRuns(ctx, providerID, modelID, &since, limit)
}

// LatestRunsByModel returns the newest chat and embedding probe for each model.
func (s *Store) LatestRunsByModel(ctx context.Context) ([]LatestRun, error) {
	rows, err := s.pool.Query(ctx, `
		WITH runs AS (
			SELECT
				id AS run_id,
				'chat' AS capability,
				provider_id,
				model_id,
				started_at,
				ok,
				status_code,
				COALESCE(request_latency_ms, latency_ms) AS latency_ms,
				ttft_ms,
				itl_ms,
				tpot_ms,
				input_tokens,
				output_tokens,
				total_tokens,
				output_tokens_per_second,
				NULL::integer AS vector_dimensions
			FROM chat_runs
			UNION ALL
			SELECT
				id AS run_id,
				'embedding' AS capability,
				provider_id,
				model_id,
				started_at,
				ok,
				status_code,
				latency_ms,
				NULL::double precision AS ttft_ms,
				NULL::double precision AS itl_ms,
				NULL::double precision AS tpot_ms,
				input_tokens,
				NULL::integer AS output_tokens,
				total_tokens,
				NULL::double precision AS output_tokens_per_second,
				vector_dimensions
			FROM embedding_runs
		)
		SELECT DISTINCT ON (provider_id, capability, model_id)
			capability,
			provider_id,
			model_id,
			started_at,
			ok,
			status_code,
			latency_ms,
			ttft_ms,
			itl_ms,
			tpot_ms,
			input_tokens,
			output_tokens,
			total_tokens,
			output_tokens_per_second,
			vector_dimensions
		FROM runs
		ORDER BY provider_id ASC, capability ASC, model_id ASC, started_at DESC, run_id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []LatestRun
	for rows.Next() {
		var run LatestRun
		var ttft pgtype.Float8
		var itl pgtype.Float8
		var tpot pgtype.Float8
		var inputTokens pgtype.Int4
		var outputTokens pgtype.Int4
		var totalTokens pgtype.Int4
		var outputTokensPerSecond pgtype.Float8
		var vectorDimensions pgtype.Int4
		if err := rows.Scan(
			&run.Capability,
			&run.ProviderID,
			&run.ModelID,
			&run.StartedAt,
			&run.OK,
			&run.StatusCode,
			&run.LatencyMS,
			&ttft,
			&itl,
			&tpot,
			&inputTokens,
			&outputTokens,
			&totalTokens,
			&outputTokensPerSecond,
			&vectorDimensions,
		); err != nil {
			return nil, err
		}
		run.TTFTMS = floatPtr(ttft)
		run.ITLMS = floatPtr(itl)
		run.TPOTMS = floatPtr(tpot)
		run.InputTokens = intPtr(inputTokens)
		run.OutputTokens = intPtr(outputTokens)
		run.TotalTokens = intPtr(totalTokens)
		run.OutputTokensPerSecond = floatPtr(outputTokensPerSecond)
		run.VectorDimensions = intPtr(vectorDimensions)
		run.ModelKey = ModelKey(run.ModelID)
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// recentRuns returns recent probes, optionally scoped to a model ID.
func (s *Store) recentRuns(ctx context.Context, providerID, modelID string, since *time.Time, limit int) ([]RecentRun, error) {
	var sinceArg any
	if since != nil {
		sinceArg = *since
	}
	rows, err := s.pool.Query(ctx, `
		SELECT capability, provider_id, model_id, prompt_id, started_at, ok, status_code, latency_ms, input_tokens, output_tokens, total_tokens, error, fixture_path, fixture_bytes, vector_dimensions
		FROM (
			SELECT 'chat' AS capability, provider_id, model_id, prompt_id, started_at, ok, status_code, latency_ms, input_tokens, output_tokens, total_tokens, error, NULL::text AS fixture_path, NULL::integer AS fixture_bytes, NULL::integer AS vector_dimensions
			FROM chat_runs
			UNION ALL
			SELECT 'embedding' AS capability, provider_id, model_id, '' AS prompt_id, started_at, ok, status_code, latency_ms, input_tokens, NULL::integer AS output_tokens, total_tokens, error, fixture_path, fixture_bytes, vector_dimensions
			FROM embedding_runs
		) runs
		WHERE ($2 = '' OR provider_id = $2)
			AND ($3 = '' OR model_id = $3)
			AND ($4::timestamptz IS NULL OR started_at >= $4)
		ORDER BY started_at DESC
		LIMIT $1
	`, limit, providerID, modelID, sinceArg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []RecentRun
	for rows.Next() {
		var run RecentRun
		var fixturePath pgtype.Text
		var fixtureBytes pgtype.Int4
		var vectorDimensions pgtype.Int4
		if err := rows.Scan(&run.Capability, &run.ProviderID, &run.ModelID, &run.PromptID, &run.StartedAt, &run.OK, &run.StatusCode, &run.LatencyMS, &run.InputTokens, &run.OutputTokens, &run.TotalTokens, &run.Error, &fixturePath, &fixtureBytes, &vectorDimensions); err != nil {
			return nil, err
		}
		run.ModelKey = ModelKey(run.ModelID)
		run.FixturePath = textPtr(fixturePath)
		run.FixtureBytes = intPtr(fixtureBytes)
		run.VectorDimensions = intPtr(vectorDimensions)
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// textPtr converts nullable PostgreSQL text into an optional JSON field.
func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

// intPtr converts nullable PostgreSQL int4 into an optional JSON field.
func intPtr(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	number := int(value.Int32)
	return &number
}

// floatPtr converts nullable PostgreSQL float8 into an optional metric value.
func floatPtr(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	number := value.Float64
	return &number
}
