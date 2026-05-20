# Configuration

Configuration is loaded from `LLM_MONITOR_CONFIG`, defaulting to `config.yaml`. Use [config.example.yaml](../config.example.yaml) as the production template and [examples/config.compose.yaml](../examples/config.compose.yaml) for local Docker Compose.

## Config Files

| File | Purpose |
| --- | --- |
| [config.example.yaml](../config.example.yaml) | Production-oriented template with all major blocks. |
| [examples/config.compose.yaml](../examples/config.compose.yaml) | Local Docker Compose config mounted by `docker-compose.yml`. |
| [examples/embedding-fixture.txt](../examples/embedding-fixture.txt) | Local embedding probe fixture. |

## Required Values

- `postgres.dsn`: PostgreSQL connection string.
- `target.base_url`: OpenAI-compatible API base URL.
- `auth.token_url`: required only when `auth.enabled` is `true`.
- `smtp.host`, `smtp.from`, and `smtp.to`: required only when `smtp.enabled` is `true`.
- `mcp.bearer_token`: required only when `mcp.enabled` is `true`.

## Secrets

Secret values are supplied inline in the config file:

- `target.api_key`
- `auth.client_secret`
- `smtp.password`
- `mcp.bearer_token`

For the Helm chart, pass these values with `--set-string config.<path>=...`.
Certificate and CA settings still use file paths because TLS libraries load them
from mounted files.

OAuth client credentials are sent with HTTP Basic authentication by default
(`auth.client_auth_method: client_secret_basic`). Set
`auth.client_auth_method: client_secret_post` only for token endpoints that
expect `client_id` and `client_secret` in the form body.

## MCP Endpoint

The optional MCP Streamable HTTP endpoint is disabled by default. Enable it with `mcp.enabled: true`, set `mcp.path` if `/mcp` is not suitable, and provide a dedicated bearer token.

## Probe Scheduling

The `schedules` block controls independent local task schedules:

- `http_check`: target reachability.
- `auth_check`: token endpoint health.
- `model_snapshot`: `/v1/models` inventory and capability detection.
- `model_runs`: scheduled chat and embedding probes.

Durations use Go strings such as `30s`, `5m`, or `24h`.
See [Scheduled Tasks](tasks/README.md) for each registered task, stable task
name, handler category, and trigger behavior.

## Target Retry

The optional `target.retry` block controls retries for outbound LLM API calls.
Retries are enabled by default with `max_retries: 2`, `wait_min: 500ms`, and
`wait_max: 5s`. Set `target.retry.enabled: false` or
`target.retry.max_retries: 0` to disable retries.

## Retention

The optional `retention` block controls automatic pruning of persisted history:

- `history`: how long historical rows are kept. Defaults to `90d` when omitted; set it explicitly to `0s` to disable pruning.

Durations also support day values such as `30d` or `90d`. When retention is enabled, dashboard KPI ranges longer than `retention.history` are capped server-side and hidden from the dashboard dropdown.

## Dashboard

The dashboard layout and chart list are static in code. Configuration controls the visible site name, optional public site link, default KPI window, and the SLO thresholds used for status badges.

- `dashboard.site_name`: display name used in the frontend header, browser title, and alert emails. Defaults to `LLM Service Monitor`.
- `dashboard.site_url`: optional absolute `http` or `https` URL for the public dashboard link shown in the frontend and alert emails.

The built-in charts are:

- Time to first token by model, rendered as a line chart.
- Request latency by model, rendered as a line chart.
- HTTP check latency, rendered as a line chart.
- Model status history, rendered as a stacked bar chart.
- Model detail latency, throughput, and error charts, rendered as bar or stacked bar charts depending on the metric.

## Related Docs

- [Deployment](deployment.md) explains how config files and certificates are passed to the container.
- [Scheduled Tasks](tasks/README.md) documents the runtime behavior controlled by `schedules`, `tests`, `models`, and `retention`.
- [Operations](operations.md) covers the operational impact of health checks, alerts, and retention.
