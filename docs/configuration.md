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
- `redis.addr`: Redis host/port used by Asynq. Defaults to `localhost:6379`.
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

## Logging

`logging.level` controls stdout verbosity. Supported levels are `debug`, `info`,
`warn`, and `error`; the default is `info`. Use `debug` when diagnosing provider
integration issues because it includes task starts/completions and per-request
endpoint/status/latency details without logging request bodies or secrets.

## Queue

Redis and Asynq back task execution:

- `redis.addr`, `redis.username`, `redis.password`, and `redis.db` configure the queue backend.
- `asynq.queue` selects the queue name. Defaults to `default`.
- `asynq.worker_concurrency` controls how many tasks a worker processes at once.
- `asynq.scheduler_sync_interval` controls how often the scheduler refreshes dynamic model schedules.
- `asynq.manual_task_retention` keeps manually triggered job status long enough for dashboard polling.

## Target Endpoints

`target.endpoints` customizes the OpenAI-like routes used by the monitor:

- `target.endpoints.models`: model inventory endpoint. Defaults to `/v1/models`.
- `target.endpoints.chat`: chat completions endpoint. Defaults to `/v1/chat/completions`.
- `target.endpoints.embeddings`: embedding endpoint. Defaults to `/v1/embeddings`.

Each endpoint can be either a path beginning with `/`, resolved against
`target.base_url`, or an absolute `http`/`https` URL. `target.http_check_path` is
still supported for existing configs; when omitted, HTTP checks use
`target.endpoints.models`.

OAuth client credentials are sent with HTTP Basic authentication by default
(`auth.client_auth_method: client_secret_basic`). Set
`auth.client_auth_method: client_secret_post` only for token endpoints that
expect `client_id` and `client_secret` in the form body.

Set `tls.insecure_skip_verify: true` to skip certificate verification for
outbound HTTP requests to the target API and OAuth token endpoint. Keep it off
unless the upstream endpoints use certificates that cannot be trusted through
`target.ca_file` or `auth.mtls.ca_file`.

## MCP Endpoint

The optional MCP Streamable HTTP endpoint is disabled by default. Enable it with `mcp.enabled: true`, set `mcp.path` if `/mcp` is not suitable, and provide a dedicated bearer token.

## Probe Scheduling

The `schedules` block controls independent Asynq schedules:

- `http_check`: target reachability.
- `auth_check`: token endpoint health.
- `model_snapshot`: model inventory and capability detection.
- `model_runs`: default interval for one-model probe tasks.
- `model_run_overrides`: optional exact `model_id` or wildcard `pattern` intervals.

Exact `model_id` overrides win over wildcard patterns, and unmatched models use
`model_runs`. Wildcards support `*` and `?`, for example `embedding-*`.

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
