# Operations

Use this page for production behavior. Use [Scheduled Tasks](tasks/README.md) for
registered task implementation details and [Configuration](configuration.md) for
the YAML fields that tune those schedules.

## Health Checks

- `GET /healthz` reports process health.
- `GET /api/status` reports target/auth/model health from persisted observations.
- `GET /metrics` exposes Prometheus metrics from the latest persisted checks, model inventory, and model probes.

Use `/healthz` for container liveness and `/api/status` for service-level monitoring.
Use `/metrics` for Prometheus scraping; it is intentionally unauthenticated and should be reachable only from an internal monitoring network.

Example scrape config:

```yaml
scrape_configs:
  - job_name: llm-service-monitor
    static_configs:
      - targets: ["llm-monitor:8080"]
```

## Alert Behavior

Model lifecycle alerts are deduplicated by alert key:

- first seen: a model appears for the first time
- missing: a model has been absent longer than `models.absence_alert_after`
- returned: a model returns after a long absence

Alert send attempts are recorded even when SMTP delivery fails, and failures also create model events.

## Troubleshooting

- No dashboard data: confirm PostgreSQL connectivity and that `model_snapshot` has run at least once.
- Auth degraded: check OAuth URL, client secret, mTLS files, CA trust, and token endpoint status.
- HTTP degraded: check `target.base_url`, `target.http_check_path`, CA trust, and network routing.
- Models skipped: inspect model event details; skipped models usually failed both embedding and chat capability probes.
- Empty embedding runs: confirm `tests.embedding_fixture.path` is mounted and readable.

## Routine Maintenance

Monitor PostgreSQL growth for run, check, snapshot, alert, and event tables. `retention.history` defaults to `90d` and prunes historical rows automatically once per day; set it explicitly to `0s` to keep all history.

## Related Docs

- [API](api.md) documents the health, status, metrics, and dashboard endpoints.
- [History retention task](tasks/history-retention.md) documents exactly which tables are pruned.
- [Model snapshot task](tasks/model-snapshot.md) documents lifecycle events and alert deduplication.
