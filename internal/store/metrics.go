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
				GREATEST(known.known_count::double precision - snapshot.active_count, 0) AS missing_count
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
				('missing', missing_count),
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
	var summary KPISummary
	var requestP50, requestP90, requestP95, requestP99 pgtype.Float8
	var ttftP50, ttftP90, ttftP95, ttftP99 pgtype.Float8
	var itlP50, itlP90, itlP95, itlP99 pgtype.Float8
	var tpotP50, tpotP90, tpotP95, tpotP99 pgtype.Float8
	var outputTokensPerSecond pgtype.Float8
	err := s.pool.QueryRow(ctx, `
		WITH runs AS (
			SELECT
				'chat' AS kind,
				model_id,
				ok,
				COALESCE(request_latency_ms, latency_ms) AS request_latency_ms,
				ttft_ms,
				itl_ms,
				tpot_ms,
				input_tokens,
				output_tokens,
				output_tokens_per_second
			FROM chat_runs WHERE started_at >= $1
			UNION ALL
			SELECT
				'embedding' AS kind,
				model_id,
				ok,
				latency_ms AS request_latency_ms,
				NULL::double precision AS ttft_ms,
				NULL::double precision AS itl_ms,
				NULL::double precision AS tpot_ms,
				input_tokens,
				NULL::integer AS output_tokens,
				NULL::double precision AS output_tokens_per_second
			FROM embedding_runs WHERE started_at >= $1
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
			COUNT(DISTINCT model_id) FILTER (WHERE slo_violation),
			COUNT(*) FILTER (WHERE NOT ok),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0)
		FROM classified
	`, since, slo.TTFTP99MS, slo.ITLP99MS, slo.RequestLatencyP99MS).Scan(
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
	query, err := metricQuery(metric, groupBy)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, query, since)
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

// metricQuery returns the SQL needed for a supported dashboard metric.
func metricQuery(metric, groupBy string) (string, error) {
	valueExpr := map[string]string{
		"latency_ms":               "request_latency_ms",
		"request_latency_ms":       "request_latency_ms",
		"ttft_ms":                  "ttft_ms",
		"itl_ms":                   "itl_ms",
		"tpot_ms":                  "tpot_ms",
		"output_tokens_per_second": "output_tokens_per_second",
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
		groupExpr = "model_id"
	case "capability":
		groupExpr = "capability"
	case "check":
		groupExpr = "model_id"
	default:
		return "", fmt.Errorf("unsupported chart group_by %q", groupBy)
	}
	if metric == "auth_latency_ms" {
		return fmt.Sprintf(`
			SELECT checked_at AS at, 'auth' AS model_id, 'auth' AS capability, 'auth' AS sample_group, %s AS value
			FROM auth_checks
			WHERE checked_at >= $1
			ORDER BY checked_at ASC
		`, valueExpr), nil
	}
	if metric == "http_latency_ms" {
		return fmt.Sprintf(`
			SELECT checked_at AS at, 'http' AS model_id, 'http' AS capability, 'http' AS sample_group, %s AS value
			FROM http_checks
			WHERE checked_at >= $1
			ORDER BY checked_at ASC
		`, valueExpr), nil
	}
	filterExpr := "TRUE"
	switch metric {
	case "ttft_ms", "itl_ms", "tpot_ms", "output_tokens_per_second":
		filterExpr = valueExpr + " IS NOT NULL"
	}
	return fmt.Sprintf(`
		WITH runs AS (
			SELECT
				c.started_at AS at,
				c.model_id,
				COALESCE(m.capability, 'chat') AS capability,
				c.ok,
				COALESCE(c.request_latency_ms, c.latency_ms) AS request_latency_ms,
				c.ttft_ms,
				c.itl_ms,
				c.tpot_ms,
				c.input_tokens,
				c.output_tokens,
				c.output_tokens_per_second
			FROM chat_runs c
			LEFT JOIN model_states m ON m.model_id = c.model_id
			WHERE c.started_at >= $1
			UNION ALL
			SELECT
				e.started_at AS at,
				e.model_id,
				COALESCE(m.capability, 'embedding') AS capability,
				e.ok,
				e.latency_ms AS request_latency_ms,
				NULL::double precision AS ttft_ms,
				NULL::double precision AS itl_ms,
				NULL::double precision AS tpot_ms,
				e.input_tokens,
				NULL::integer AS output_tokens,
				NULL::double precision AS output_tokens_per_second
			FROM embedding_runs e
			LEFT JOIN model_states m ON m.model_id = e.model_id
			WHERE e.started_at >= $1
		)
		SELECT at, model_id, capability, %s AS sample_group, %s AS value
		FROM runs
		WHERE %s
		ORDER BY at ASC
	`, groupExpr, valueExpr, filterExpr), nil
}
