# API

The Go service exposes JSON APIs for health and dashboard data, plus a static SPA fallback.

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
  "missing_models": 0,
  "skipped_models": 1,
  "auth_ok": true,
  "http_ok": true
}
```

## `GET /api/dashboard`

Returns the full dashboard payload. Optional query parameter:

- `range`: Go duration string such as `24h`, `168h`, `720h`, or `8760h`. When `retention.history` is enabled, windows longer than the retention period are capped.

The response includes generated time, status, KPIs, SLOs, static dashboard charts, model status history, current models, recent events, recent runs, recent alerts, latest auth/HTTP checks, and non-secret runtime config such as `config.site_name`, `config.site_url`, and `config.retention.history_seconds`. Chart types are `line`, `bar`, or `stacked-bar`; dataset values can be `null` when a bucket has no sample.

## `GET /api/model-dashboard`

Returns KPI, chart, and recent probe telemetry for one model. Query parameters:

- `model_id`: required
- `range`: optional Go duration string such as `24h`, `168h`, `720h`, or `8760h`; capped by `retention.history` when retention is enabled

```text
/api/model-dashboard?model_id=gpt-test&range=24h
```

The response includes generated time, the current model state, model-scoped KPIs, SLOs, model-scoped charts, and recent runs in the selected window. Chart types are `line`, `bar`, or `stacked-bar`; dataset values can be `null` when a bucket has no sample. Missing `model_id` returns `400`; unknown models return `404`.

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

## `/mcp`

When `mcp.enabled` is `true`, the service exposes a Streamable HTTP MCP endpoint at `mcp.path`, defaulting to `/mcp`. Requests must include `Authorization: Bearer <mcp token>`.

The v1 MCP server exposes only read-only tools:

- `llm_monitor.status`
- `llm_monitor.kpis`
- `llm_monitor.models`
- `llm_monitor.model_performance`
