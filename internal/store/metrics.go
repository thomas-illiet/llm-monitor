package store

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ModelStatusSamples loads snapshot-level model counts grouped by dashboard status.
func (s *Store) ModelStatusSamples(ctx context.Context, since time.Time) ([]MetricSample, error) {
	rows, err := s.pool.Query(ctx, `
		WITH snapshot_counts AS (
			SELECT
				s.id,
				s.observed_at,
				COUNT(i.model_id)::double precision AS active_count,
				COUNT(i.model_id) FILTER (WHERE i.excluded OR i.capability = 'skip')::double precision AS skipped_count
			FROM model_snapshots s
			LEFT JOIN model_snapshot_items i ON i.snapshot_id = s.id
			WHERE s.observed_at >= $1
			GROUP BY s.id, s.observed_at
		),
		first_seen AS (
			SELECT item.model_id, MIN(snapshot.observed_at) AS first_seen_at
			FROM model_snapshot_items item
			JOIN model_snapshots snapshot ON snapshot.id = item.snapshot_id
			GROUP BY item.model_id
		),
		status_counts AS (
			SELECT
				snapshot.observed_at AS at,
				snapshot.active_count,
				snapshot.skipped_count,
				GREATEST(known.known_count::double precision - snapshot.active_count, 0) AS inactive_count
			FROM snapshot_counts snapshot
			CROSS JOIN LATERAL (
				SELECT COUNT(*) AS known_count
				FROM first_seen
				WHERE first_seen_at <= snapshot.observed_at
			) known
		)
		SELECT at, 'model_inventory' AS model_id, 'inventory' AS capability, sample_group, value
		FROM status_counts
		CROSS JOIN LATERAL (
			VALUES
				('active', active_count),
				('inactive', inactive_count),
				('skipped', skipped_count)
		) AS samples(sample_group, value)
		ORDER BY at ASC, sample_group ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var samples []MetricSample
	for rows.Next() {
		var sample MetricSample
		if err := rows.Scan(&sample.At, &sample.ModelID, &sample.Capability, &sample.Group, &sample.Value); err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}

// KPISummary aggregates recent GuideLLM-style latency, throughput, SLO, and token metrics.
func (s *Store) KPISummary(ctx context.Context, since time.Time, slo SLOThresholds) (KPISummary, error) {
	return s.kpiSummary(ctx, since, slo, "", "")
}

// KPISummaryForModel aggregates recent latency, throughput, SLO, and token metrics for one model.
func (s *Store) KPISummaryForModel(ctx context.Context, providerID, modelID string, since time.Time, slo SLOThresholds) (KPISummary, error) {
	return s.kpiSummary(ctx, since, slo, providerID, modelID)
}

// kpiSummary runs the KPI aggregate query, optionally scoped to a model ID.
func (s *Store) kpiSummary(ctx context.Context, since time.Time, slo SLOThresholds, providerID, modelID string) (KPISummary, error) {
	var summary KPISummary
	var requestP50, requestP90, requestP95, requestP99 pgtype.Float8
	var ttftP50, ttftP90, ttftP95, ttftP99 pgtype.Float8
	var itlP50, itlP90, itlP95, itlP99 pgtype.Float8
	var tpotP50, tpotP90, tpotP95, tpotP99 pgtype.Float8
	var outputTokensPerSecond pgtype.Float8
	err := s.pool.QueryRow(ctx, `
			WITH runs AS (
				SELECT
					provider_id,
					model_id,
					ok,
					COALESCE(request_latency_ms, latency_ms) AS request_latency_ms,
				ttft_ms,
				itl_ms,
				tpot_ms,
				input_tokens,
				output_tokens,
				output_tokens_per_second
				FROM chat_runs WHERE started_at >= $1 AND ($5 = '' OR provider_id = $5) AND ($6 = '' OR model_id = $6)
				UNION ALL
				SELECT
					provider_id,
					model_id,
					ok,
					latency_ms AS request_latency_ms,
				NULL::double precision AS ttft_ms,
				NULL::double precision AS itl_ms,
				NULL::double precision AS tpot_ms,
				input_tokens,
				NULL::integer AS output_tokens,
				NULL::double precision AS output_tokens_per_second
			FROM embedding_runs WHERE started_at >= $1 AND ($5 = '' OR provider_id = $5) AND ($6 = '' OR model_id = $6)
		),
		classified AS (
			SELECT *,
				(
					NOT ok OR
					request_latency_ms > $4 OR
					(ttft_ms IS NOT NULL AND ttft_ms > $2) OR
					(itl_ms IS NOT NULL AND itl_ms > $3)
				) AS slo_violation
			FROM runs
		)
		SELECT
			COUNT(*),
			COALESCE(AVG(CASE WHEN ok THEN 1.0 ELSE 0.0 END), 0),
			percentile_cont(0.50) WITHIN GROUP (ORDER BY request_latency_ms),
			percentile_cont(0.90) WITHIN GROUP (ORDER BY request_latency_ms),
			percentile_cont(0.95) WITHIN GROUP (ORDER BY request_latency_ms),
			percentile_cont(0.99) WITHIN GROUP (ORDER BY request_latency_ms),
			percentile_cont(0.50) WITHIN GROUP (ORDER BY ttft_ms) FILTER (WHERE ttft_ms IS NOT NULL),
			percentile_cont(0.90) WITHIN GROUP (ORDER BY ttft_ms) FILTER (WHERE ttft_ms IS NOT NULL),
			percentile_cont(0.95) WITHIN GROUP (ORDER BY ttft_ms) FILTER (WHERE ttft_ms IS NOT NULL),
			percentile_cont(0.99) WITHIN GROUP (ORDER BY ttft_ms) FILTER (WHERE ttft_ms IS NOT NULL),
			percentile_cont(0.50) WITHIN GROUP (ORDER BY itl_ms) FILTER (WHERE itl_ms IS NOT NULL),
			percentile_cont(0.90) WITHIN GROUP (ORDER BY itl_ms) FILTER (WHERE itl_ms IS NOT NULL),
			percentile_cont(0.95) WITHIN GROUP (ORDER BY itl_ms) FILTER (WHERE itl_ms IS NOT NULL),
			percentile_cont(0.99) WITHIN GROUP (ORDER BY itl_ms) FILTER (WHERE itl_ms IS NOT NULL),
			percentile_cont(0.50) WITHIN GROUP (ORDER BY tpot_ms) FILTER (WHERE tpot_ms IS NOT NULL),
			percentile_cont(0.90) WITHIN GROUP (ORDER BY tpot_ms) FILTER (WHERE tpot_ms IS NOT NULL),
			percentile_cont(0.95) WITHIN GROUP (ORDER BY tpot_ms) FILTER (WHERE tpot_ms IS NOT NULL),
			percentile_cont(0.99) WITHIN GROUP (ORDER BY tpot_ms) FILTER (WHERE tpot_ms IS NOT NULL),
			AVG(output_tokens_per_second) FILTER (WHERE output_tokens_per_second IS NOT NULL),
			COUNT(*) FILTER (WHERE slo_violation),
			COUNT(DISTINCT provider_id || ':' || model_id) FILTER (WHERE slo_violation),
			COUNT(*) FILTER (WHERE NOT ok),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0)
		FROM classified
	`, since, slo.TTFTP99MS, slo.ITLP99MS, slo.RequestLatencyP99MS, providerID, modelID).Scan(
		&summary.TotalRuns,
		&summary.SuccessRate,
		&requestP50,
		&requestP90,
		&requestP95,
		&requestP99,
		&ttftP50,
		&ttftP90,
		&ttftP95,
		&ttftP99,
		&itlP50,
		&itlP90,
		&itlP95,
		&itlP99,
		&tpotP50,
		&tpotP90,
		&tpotP95,
		&tpotP99,
		&outputTokensPerSecond,
		&summary.SLOViolationCount,
		&summary.DegradedModels,
		&summary.ErrorCount,
		&summary.InputTokens,
		&summary.OutputTokens,
	)
	if err != nil {
		return summary, err
	}
	summary.RequestLatencyP50MS = safeFloat(requestP50)
	summary.RequestLatencyP90MS = safeFloat(requestP90)
	summary.RequestLatencyP95MS = safeFloat(requestP95)
	summary.RequestLatencyP99MS = safeFloat(requestP99)
	summary.LatencyP50MS = summary.RequestLatencyP50MS
	summary.LatencyP95MS = summary.RequestLatencyP95MS
	summary.LatencyP99MS = summary.RequestLatencyP99MS
	summary.TTFTP50MS = safeFloat(ttftP50)
	summary.TTFTP90MS = safeFloat(ttftP90)
	summary.TTFTP95MS = safeFloat(ttftP95)
	summary.TTFTP99MS = safeFloat(ttftP99)
	summary.ITLP50MS = safeFloat(itlP50)
	summary.ITLP90MS = safeFloat(itlP90)
	summary.ITLP95MS = safeFloat(itlP95)
	summary.ITLP99MS = safeFloat(itlP99)
	summary.TPOTP50MS = safeFloat(tpotP50)
	summary.TPOTP90MS = safeFloat(tpotP90)
	summary.TPOTP95MS = safeFloat(tpotP95)
	summary.TPOTP99MS = safeFloat(tpotP99)
	summary.OutputTokensPerSecond = safeFloat(outputTokensPerSecond)
	return summary, nil
}

// safeFloat converts nullable percentile values into dashboard-safe numbers.
func safeFloat(value pgtype.Float8) float64 {
	if !value.Valid || math.IsNaN(value.Float64) {
		return 0
	}
	return value.Float64
}

// MetricSamples loads raw metric points used to build configured chart series.
func (s *Store) MetricSamples(ctx context.Context, metric, groupBy string, since time.Time) ([]MetricSample, error) {
	return s.metricSamples(ctx, metric, groupBy, since, "", "")
}

// MetricSamplesForModel loads raw metric points for one model.
func (s *Store) MetricSamplesForModel(ctx context.Context, metric, groupBy string, since time.Time, providerID, modelID string) ([]MetricSample, error) {
	return s.metricSamples(ctx, metric, groupBy, since, providerID, modelID)
}

// metricSamples loads raw metric points, optionally scoped to a model ID.
func (s *Store) metricSamples(ctx context.Context, metric, groupBy string, since time.Time, providerID, modelID string) ([]MetricSample, error) {
	query, err := metricQuery(metric, groupBy)
	args := []any{since}
	if modelID != "" {
		query, err = metricQueryForModel(metric, groupBy)
		args = append(args, providerID, modelID)
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var samples []MetricSample
	for rows.Next() {
		var sample MetricSample
		if err := rows.Scan(&sample.At, &sample.ProviderID, &sample.ModelID, &sample.Capability, &sample.Group, &sample.Value); err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}

// ModelPerformance summarizes recent chat and embedding run performance by model.
func (s *Store) ModelPerformance(ctx context.Context, query ModelPerformanceQuery) ([]ModelPerformanceRow, error) {
	if query.Limit <= 0 {
		query.Limit = 20
	}
	orderBy := modelPerformanceOrderBy(query.Sort)
	rows, err := s.pool.Query(ctx, `
		WITH runs AS (
			SELECT
				provider_id,
				model_id,
				started_at,
				ok,
				status_code,
				COALESCE(request_latency_ms, latency_ms) AS latency_ms,
				error
			FROM chat_runs
			WHERE started_at >= $1
			UNION ALL
			SELECT
				provider_id,
				model_id,
				started_at,
				ok,
				status_code,
				latency_ms,
				error
			FROM embedding_runs
			WHERE started_at >= $1
		),
		aggregated AS (
			SELECT
				provider_id,
				model_id,
				COUNT(*) AS runs,
				COALESCE(AVG(CASE WHEN ok THEN 1.0 ELSE 0.0 END), 0) AS success_rate,
				COUNT(*) FILTER (WHERE NOT ok) AS error_count,
				AVG(latency_ms) AS avg_latency_ms,
				percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms) AS p50_latency_ms,
				percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95_latency_ms,
				percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms) AS p99_latency_ms,
				MIN(started_at) AS first_run_at,
				MAX(started_at) AS last_run_at
			FROM runs
			GROUP BY provider_id, model_id
		),
		latest_errors AS (
			SELECT DISTINCT ON (provider_id, model_id)
				provider_id,
				model_id,
				status_code,
				error
			FROM runs
			WHERE NOT ok
			ORDER BY provider_id, model_id, started_at DESC
		)
		SELECT
			aggregated.provider_id,
			aggregated.model_id,
			aggregated.runs,
			aggregated.success_rate,
			aggregated.error_count,
			aggregated.avg_latency_ms,
			aggregated.p50_latency_ms,
			aggregated.p95_latency_ms,
			aggregated.p99_latency_ms,
			aggregated.first_run_at,
			aggregated.last_run_at,
			latest_errors.status_code,
			latest_errors.error
		FROM aggregated
		LEFT JOIN latest_errors ON latest_errors.provider_id = aggregated.provider_id AND latest_errors.model_id = aggregated.model_id
		ORDER BY `+orderBy+`
		LIMIT $2
	`, query.Since, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ModelPerformanceRow
	for rows.Next() {
		var row ModelPerformanceRow
		var avgLatency, p50Latency, p95Latency, p99Latency pgtype.Float8
		var latestStatus pgtype.Int4
		var latestError pgtype.Text
		if err := rows.Scan(
			&row.ProviderID,
			&row.ModelID,
			&row.Runs,
			&row.SuccessRate,
			&row.ErrorCount,
			&avgLatency,
			&p50Latency,
			&p95Latency,
			&p99Latency,
			&row.FirstRunAt,
			&row.LastRunAt,
			&latestStatus,
			&latestError,
		); err != nil {
			return nil, err
		}
		row.AvgLatencyMS = safeFloat(avgLatency)
		row.P50LatencyMS = safeFloat(p50Latency)
		row.P95LatencyMS = safeFloat(p95Latency)
		row.P99LatencyMS = safeFloat(p99Latency)
		row.ModelKey = ModelKey(row.ModelID)
		if latestStatus.Valid || latestError.Valid {
			row.LatestError = &ModelPerformanceError{
				StatusCode: int(latestStatus.Int32),
				Message:    latestError.String,
			}
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// modelPerformanceOrderBy returns a safe ORDER BY fragment for supported sorts.
func modelPerformanceOrderBy(sort string) string {
	switch sort {
	case "success_rate":
		return "success_rate DESC, error_count ASC, model_id ASC"
	case "avg_latency_ms":
		return "avg_latency_ms DESC, error_count DESC, model_id ASC"
	case "p95_latency_ms":
		return "p95_latency_ms DESC, error_count DESC, model_id ASC"
	case "p99_latency_ms":
		return "p99_latency_ms DESC, error_count DESC, model_id ASC"
	case "model_id":
		return "model_id ASC"
	default:
		return "error_count DESC, success_rate ASC, model_id ASC"
	}
}

// metricQuery returns the SQL needed for a supported dashboard metric.
func metricQuery(metric, groupBy string) (string, error) {
	return metricQueryWithModelScope(metric, groupBy, false)
}

// metricQueryForModel returns the SQL needed for a model-scoped dashboard metric.
func metricQueryForModel(metric, groupBy string) (string, error) {
	return metricQueryWithModelScope(metric, groupBy, true)
}

// metricQueryWithModelScope builds chart sample SQL and can constrain run-backed metrics to one model.
func metricQueryWithModelScope(metric, groupBy string, modelScoped bool) (string, error) {
	valueExpr := map[string]string{
		"latency_ms":               "request_latency_ms",
		"request_latency_ms":       "request_latency_ms",
		"ttft_ms":                  "ttft_ms",
		"itl_ms":                   "itl_ms",
		"tpot_ms":                  "tpot_ms",
		"output_tokens_per_second": "output_tokens_per_second",
		"vector_dimensions":        "vector_dimensions::double precision",
		"success_rate":             "CASE WHEN ok THEN 1.0 ELSE 0.0 END",
		"errors":                   "CASE WHEN ok THEN 0.0 ELSE 1.0 END",
		"input_tokens":             "COALESCE(input_tokens, 0)::double precision",
		"output_tokens":            "COALESCE(output_tokens, 0)::double precision",
		"auth_latency_ms":          "latency_ms",
		"http_latency_ms":          "latency_ms",
	}[metric]
	if valueExpr == "" {
		return "", fmt.Errorf("unsupported chart metric %q", metric)
	}
	groupExpr := "'all'"
	switch groupBy {
	case "", "none":
		groupExpr = "'all'"
	case "model":
		groupExpr = "provider_id || '/' || model_id"
	case "capability":
		groupExpr = "capability"
	case "check":
		groupExpr = "model_id"
	default:
		return "", fmt.Errorf("unsupported chart group_by %q", groupBy)
	}
	if metric == "auth_latency_ms" {
		if modelScoped {
			return "", fmt.Errorf("unsupported model-scoped chart metric %q", metric)
		}
		return fmt.Sprintf(`
			SELECT checked_at AS at, provider_id, provider_id AS model_id, 'auth' AS capability, provider_id AS sample_group, %s AS value
			FROM auth_checks
			WHERE checked_at >= $1
			ORDER BY checked_at ASC
		`, valueExpr), nil
	}
	if metric == "http_latency_ms" {
		if modelScoped {
			return "", fmt.Errorf("unsupported model-scoped chart metric %q", metric)
		}
		return fmt.Sprintf(`
			SELECT checked_at AS at, provider_id, provider_id AS model_id, 'http' AS capability, provider_id AS sample_group, %s AS value
			FROM http_checks
			WHERE checked_at >= $1
			ORDER BY checked_at ASC
		`, valueExpr), nil
	}
	filterExpr := "TRUE"
	switch metric {
	case "ttft_ms", "itl_ms", "tpot_ms", "output_tokens_per_second", "vector_dimensions":
		filterExpr = valueExpr + " IS NOT NULL"
	}
	chatWhere := "c.started_at >= $1"
	embeddingWhere := "e.started_at >= $1"
	if modelScoped {
		chatWhere += " AND c.provider_id = $2 AND c.model_id = $3"
		embeddingWhere += " AND e.provider_id = $2 AND e.model_id = $3"
	}
	return fmt.Sprintf(`
		WITH runs AS (
			SELECT
				c.started_at AS at,
				c.provider_id,
				c.model_id,
				COALESCE(m.capability, 'chat') AS capability,
				c.ok,
				COALESCE(c.request_latency_ms, c.latency_ms) AS request_latency_ms,
				c.ttft_ms,
				c.itl_ms,
				c.tpot_ms,
				c.input_tokens,
				c.output_tokens,
				c.output_tokens_per_second,
				NULL::integer AS vector_dimensions
			FROM chat_runs c
			LEFT JOIN model_states m ON m.provider_id = c.provider_id AND m.model_id = c.model_id
			WHERE %s
			UNION ALL
			SELECT
				e.started_at AS at,
				e.provider_id,
				e.model_id,
				COALESCE(m.capability, 'embedding') AS capability,
				e.ok,
				e.latency_ms AS request_latency_ms,
				NULL::double precision AS ttft_ms,
				NULL::double precision AS itl_ms,
				NULL::double precision AS tpot_ms,
				e.input_tokens,
				NULL::integer AS output_tokens,
				NULL::double precision AS output_tokens_per_second,
				e.vector_dimensions
			FROM embedding_runs e
			LEFT JOIN model_states m ON m.provider_id = e.provider_id AND m.model_id = e.model_id
			WHERE %s
		)
		SELECT at, provider_id, model_id, capability, %s AS sample_group, %s AS value
		FROM runs
		WHERE %s
		ORDER BY at ASC
	`, chatWhere, embeddingWhere, groupExpr, valueExpr, filterExpr), nil
}
