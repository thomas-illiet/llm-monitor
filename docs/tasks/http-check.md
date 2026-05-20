# HTTP Check Task

## Purpose

`monitor.http_check` verifies that the configured OpenAI-compatible target is
reachable over HTTP. It is the lightest availability signal for the monitored LLM
service.

## Schedule

- Config key: `schedules.http_check`
- Default interval: `30s`
- Startup behavior: runs once immediately, then repeats on the configured interval.
- Payload: empty JSON payload.

## Inputs

- `target.base_url`: base URL for the monitored LLM API.
- `target.http_check_path`: optional path or absolute URL requested on the target.
  When omitted, it defaults to `target.endpoints.models`.
- `target.endpoints.models`: default model inventory endpoint, used by HTTP
  checks when `target.http_check_path` is not set.
- `target.timeout`, `target.ca_file`, `target.retry`, and target authentication settings used by the LLM HTTP client.

## Execution

The handler from `internal/schedule/tasks/checks/http_check.go` calls
`LLMClient.HealthCheck`, which sends a `GET` request to the configured health
path. Responses with status codes from `200` through `399` are treated as
healthy. Failed requests use the configured retry policy before the final result
is recorded.

The response body is read with a small bound and discarded; this task records
reachability, status, latency, and a compact error message.

## Stored Output

Results are inserted into `http_checks` via `RecordHTTPCheck`:

- `checked_at`
- `ok`
- `status_code`
- `latency_ms`
- `error`

The latest HTTP check feeds `/api/status`, `/metrics`, and dashboard status views.

## Failure Behavior

Network errors, request creation errors, TLS errors, or non-healthy HTTP status
codes are recorded as failed checks. After a failed final check, currently
runnable models are marked inactive and the in-memory model plan is cleared. If
persistence fails, the task returns the storage error; the local scheduler logs it
and continues on the next tick.

## Related Code

- [`internal/schedule/tasks/checks/http_check.go`](../../internal/schedule/tasks/checks/http_check.go)
- [`internal/schedule/runner`](../../internal/schedule/runner)
- [`internal/llm/models.go`](../../internal/llm/models.go)
- [`internal/store/checks.go`](../../internal/store/checks.go)
