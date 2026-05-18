# HTTP Check Task

## Purpose

`RunHTTPCheck` verifies that the configured OpenAI-compatible target is reachable
over HTTP. It is the lightest availability signal for the monitored LLM service.

## Schedule

- Config key: `schedules.http_check`
- Default interval: `30s`
- Startup behavior: runs once immediately, then repeats on the configured interval.

## Inputs

- `target.base_url`: base URL for the monitored LLM API.
- `target.http_check_path`: path requested on the target, defaulting to `/v1/models`.
- `target.timeout`, `target.ca_file`, and target authentication settings used by the LLM HTTP client.

## Execution

The task calls `LLMClient.HealthCheck`, which sends a `GET` request to the
configured health path. Responses with status codes from `200` through `399` are
treated as healthy.

The response body is discarded after a small bounded read; this task only records
reachability and latency.

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
codes are recorded as failed checks. If persistence fails, the scheduler logs the
storage error and the task returns it; the recurring loop continues on the next tick.

## Related Code

- [`internal/monitor/checks.go`](../../internal/monitor/checks.go)
- [`internal/llm/models.go`](../../internal/llm/models.go)
- [`internal/store/checks.go`](../../internal/store/checks.go)
