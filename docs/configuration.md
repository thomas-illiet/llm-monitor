# Configuration

Configuration is loaded from `LLM_MONITOR_CONFIG`, defaulting to `config.yaml`. Use [config.example.yaml](../config.example.yaml) as the production template and [examples/config.compose.yaml](../examples/config.compose.yaml) for local Docker Compose.

## Required Values

- `postgres.dsn`: PostgreSQL connection string.
- `target.base_url`: OpenAI-compatible API base URL.
- `auth.token_url`: required only when `auth.enabled` is `true`.
- `smtp.host`, `smtp.from`, and `smtp.to`: required only when `smtp.enabled` is `true`.

## Secrets

Secrets can be supplied inline for local use or via file fields for deployment:

- `target.api_key_file`
- `auth.client_secret_file`
- `smtp.password_file`

Prefer file fields in production so Docker/Kubernetes secrets can be mounted read-only.

## Probe Scheduling

The `schedules` block controls independent loops:

- `http_check`: target reachability.
- `auth_check`: token endpoint health.
- `model_snapshot`: `/v1/models` inventory and capability detection.
- `model_runs`: scheduled chat and embedding probes.

Durations use Go strings such as `30s`, `5m`, or `24h`.

## Dashboard

The dashboard layout and chart list are static in code. Configuration only controls the default KPI window and the SLO thresholds used for status badges.

The built-in charts are:

- Time to first token by model.
- Request latency by model.
- HTTP check latency.
