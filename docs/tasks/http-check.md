# HTTP Check Task

## Purpose

`monitor.http_check` verifies that each configured OpenAI-compatible provider is
reachable over HTTP. It is the lightest availability signal for monitored LLM
services.

## Schedule

- Config key: `schedules.http_check`
- Default interval: `30s`
- Startup behavior: runs once immediately, then repeats on the configured interval.
- Payload: empty JSON payload for all providers, or `provider_id` for one provider.

## Inputs

- `providers[].base_url`: base URL for the monitored LLM API.
- `providers[].http_check_path`: optional path or absolute URL requested for the provider. When omitted, it defaults to `providers[].endpoints.models`.
- `providers[].endpoints.models`: default model inventory endpoint, used by HTTP checks when `http_check_path` is not set.
- `providers[].timeout`, `providers[].ca_file`, `providers[].retry`, and provider authentication settings used by the LLM HTTP client.

## Execution

The handler from `internal/schedule/tasks/checks/http_check.go` calls
`LLMClient.HealthCheck` for each selected provider, which sends a `GET` request
to the configured health path. Responses with status codes from `200` through `399` are treated as
healthy. Failed requests use the configured retry policy before the final result
is recorded.

The response body is read with a small bound and discarded; this task records
reachability, status, latency, and a compact error message.

## Stored Output

Results are inserted into `http_checks` via `RecordHTTPCheck`:

- `provider_id`
- `checked_at`
- `ok`
- `status_code`
- `latency_ms`
- `error`

The latest HTTP check feeds `/api/status`, `/metrics`, and dashboard status views.

## Failure Behavior

Network errors, request creation errors, TLS errors, or non-healthy HTTP status
codes are recorded as failed checks. After a failed final check, currently
runnable models for that provider are marked inactive. If persistence fails, the
task returns the storage error so the worker records the failure.

## Related Code

- [`internal/schedule/tasks/checks/http_check.go`](../../internal/schedule/tasks/checks/http_check.go)
- [`internal/schedule/runner`](../../internal/schedule/runner)
- [`internal/llm/models.go`](../../internal/llm/models.go)
- [`internal/store/checks.go`](../../internal/store/checks.go)
