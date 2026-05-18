# Auth Check Task

## Purpose

`RunAuthCheck` verifies authentication health for the monitor. When OAuth is
enabled, it checks that the token endpoint can issue a client-credentials token.
When OAuth is disabled, it records static authentication as healthy because no
token endpoint is configured.

## Schedule

- Config key: `schedules.auth_check`
- Default interval: `60s`
- Startup behavior: runs once immediately, then repeats on the configured interval.

## Inputs

- `auth.enabled`: selects OAuth client credentials or static target API key mode.
- `auth.token_url`, `auth.client_id`, `auth.client_secret` or `auth.client_secret_file`.
- `auth.scopes`, `auth.audience`, `auth.timeout`, and `auth.refresh_skew`.
- `auth.mtls.*` when the token endpoint requires mutual TLS.
- `target.api_key` or `target.api_key_file` when `auth.enabled` is `false`.

## Execution

With OAuth enabled, the provider performs a token request using the
`client_credentials` grant. A successful response updates the cached bearer token
and captures the token expiration time.

With OAuth disabled, the static provider returns a successful check without making
an outbound token request.

## Stored Output

Results are inserted into `auth_checks` via `RecordAuthCheck`:

- `checked_at`
- `ok`
- `status_code`
- `latency_ms`
- `expires_at`
- `error`

The latest auth check feeds `/api/status`, `/metrics`, and dashboard status views.

## Failure Behavior

Token endpoint errors, TLS errors, invalid JSON, missing `access_token`, and
non-2xx OAuth responses are captured in the check result. If persistence fails,
the scheduler logs the storage error and the task returns it; the recurring loop
continues on the next tick.

## Related Code

- [`internal/monitor/checks.go`](../../internal/monitor/checks.go)
- [`internal/auth/provider.go`](../../internal/auth/provider.go)
- [`internal/store/checks.go`](../../internal/store/checks.go)
