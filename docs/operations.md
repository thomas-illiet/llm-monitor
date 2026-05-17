# Operations

## Health Checks

- `GET /healthz` reports process health.
- `GET /api/status` reports target/auth/model health from persisted observations.

Use `/healthz` for container liveness and `/api/status` for service-level monitoring.

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

Monitor PostgreSQL growth for run and event tables. The current schema does not prune history automatically; use external retention policies when long-lived deployments generate high probe volume.
