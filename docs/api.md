# API

The Go service exposes JSON APIs for health and dashboard data, plus a static SPA fallback.

## Endpoint Summary

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Process liveness. |
| `GET /api/status` | Compact service health for target, auth, and model inventory state. |
| `GET /metrics` | Prometheus metrics from persisted observations. |
| `GET /api/dashboard` | Full dashboard payload. |
| `GET /api/model-dashboard` | Model-specific KPIs, charts, and recent runs. |
| `GET /api/models/{model_id}/details` | Current model state and redacted provider metadata. |
| `GET /api/model-events` | Paginated model event timeline. |
| `POST /api/checks/run` | Enqueue manual global or model-specific checks. |
| `GET /api/checks/jobs` | Poll retained Asynq task status for manual checks. |
| `mcp.path`, default `/mcp` | Optional read-only MCP Streamable HTTP endpoint. |

## `GET /healthz`

Returns process health.

```json
{ "status": "ok" }
```

## `GET /api/status`

Returns compact service status, including model inventory counts and latest auth/HTTP check state.

```json
{
  "ok": true,
  "generated_at": "2026-05-17T10:00:00Z",
  "active_models": 4,
  "inactive_models": 0,
  "missing_models": 0,
  "skipped_models": 1,
  "auth_ok": true,
  "http_ok": true
}
```

`missing_models` is retained as a compatibility alias for `inactive_models`.

## `GET /metrics`

Returns Prometheus text metrics for the latest persisted monitor observations. Scraping this endpoint does not call the LLM provider.

Key metric families include:

- `llm_monitor_http_*` and `llm_monitor_auth_*` for the latest service checks
- `llm_monitor_models_total{status=...}` and `llm_monitor_models_skipped_total` for inventory counts
- `llm_monitor_model_available{model=...,capability=...}` and timestamp gauges for per-model availability
- `llm_monitor_model_probe_*{model=...,capability=...}` for the latest chat or embedding probe telemetry

The endpoint is open by design for internal Prometheus or Kubernetes scrape targets. It also includes the standard Go runtime and process collectors.

## `GET /api/dashboard`

Returns the full dashboard payload. Optional query parameter:

- `range`: Go duration string such as `24h`, `168h`, `720h`, or `8760h`. When `retention.history` is enabled, windows longer than the retention period are capped.

The response includes generated time, status, KPIs, SLOs, static dashboard charts, model status history, current models, recent events, recent runs, recent alerts, latest auth/HTTP checks, and non-secret runtime config such as `config.site_name`, `config.site_url`, and `config.retention.history_seconds`. Model rows include `next_check_at` when the Asynq scheduler heartbeat exposes a next `monitor.model_run` enqueue for that model. Chart types are `line`, `bar`, or `stacked-bar`; dataset values can be `null` when a bucket has no sample.

## `GET /api/model-dashboard`

Returns KPI, chart, and recent probe telemetry for one model. Query parameters:

- `model_id`: required
- `range`: optional Go duration string such as `24h`, `168h`, `720h`, or `8760h`; capped by `retention.history` when retention is enabled

```text
/api/model-dashboard?model_id=gpt-test&range=24h
```

The response includes generated time, the current model state, model-scoped KPIs, SLOs, model-scoped charts, and recent runs in the selected window. Chart types are `line`, `bar`, or `stacked-bar`; dataset values can be `null` when a bucket has no sample. Missing `model_id` returns `400`; unknown models return `404`.

## `GET /api/models/{model_id}/details`

Returns the current model state and provider metadata captured from the latest
`/v1/models` inventory snapshot. Encode model IDs as URL path segments, for
example:

```text
/api/models/provider%2Fmodel/details
```

The response includes `generated_at`, `model`, and `provider_metadata`. Sensitive
metadata keys such as API keys, tokens, secrets, passwords, authorization values,
credentials, and bearer values are redacted before persistence. Unknown models
return `404`.

## `GET /api/model-events`

Returns a paginated model event timeline. Query parameters:

- `model_id`: required
- `limit`: optional, default `25`, max `100`
- `offset`: optional, default `0`
- `status`: repeatable
- `source`: repeatable
- `event_type`: repeatable

```text
/api/model-events?model_id=gpt-test&status=error&source=scheduled_run
```

Invalid or missing `model_id` returns `400`; server-side failures return `500`.

## `POST /api/checks/run`

Enqueues manual work and returns `202 Accepted` with retained task IDs for
dashboard polling.

Global checks enqueue HTTP check, auth check, model snapshot, and one model run
for each currently runnable model:

```json
{ "scope": "all" }
```

One-model checks enqueue a single `monitor.model_run` task:

```json
{ "scope": "model", "model_id": "gpt-test" }
```

## `GET /api/checks/jobs`

Polls retained queue status for manual jobs. Pass a comma-separated `ids`
parameter:

```text
/api/checks/jobs?ids=job-a,job-b
```

The response contains each job state, task type, optional `model_id`, last error,
and completion timestamp when available.

## `/mcp`

When `mcp.enabled` is `true`, the service exposes a Streamable HTTP MCP endpoint at `mcp.path`, defaulting to `/mcp`. Requests must include `Authorization: Bearer <mcp token>`.

The v1 MCP server exposes only read-only tools:

- `llm_monitor.status`
- `llm_monitor.kpis`
- `llm_monitor.models`
- `llm_monitor.model_performance`

## Related Docs

- [Operations](operations.md) covers how `/healthz`, `/api/status`, and `/metrics` should be used in production.
- [Configuration](configuration.md) covers `mcp.enabled`, `mcp.path`, and MCP bearer token settings.
