# Auth Check Task

## Purpose

`monitor.auth_check` verifies authentication health for the monitor. When OAuth
is enabled, it checks that the token endpoint can issue a client-credentials
token. When OAuth is disabled, it records static authentication as healthy
because no token endpoint is configured.

## Schedule

- Config key: `schedules.auth_check`
- Default interval: `60s`
- Startup behavior: runs once immediately, then repeats on the configured interval.
- Payload: empty JSON payload.

## Inputs

- `auth.enabled`: selects OAuth client credentials or static target API key mode.
- `auth.token_url`, `auth.client_id`, `auth.client_secret`, and `auth.client_auth_method`.
- `auth.scopes`, `auth.audience`, `auth.timeout`, and `auth.refresh_skew`.
- `auth.mtls.*` when the token endpoint requires mutual TLS.
- `target.api_key` when `auth.enabled` is `false`.

## Execution

The handler from `internal/schedule/tasks/checks/auth_check.go` delegates to the
configured auth provider. With OAuth enabled, the provider performs a token
request using the
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
the task returns the storage error so the worker records the failure.

## Related Code

- [`internal/schedule/tasks/checks/auth_check.go`](../../internal/schedule/tasks/checks/auth_check.go)
- [`internal/schedule/runner`](../../internal/schedule/runner)
- [`internal/auth/provider.go`](../../internal/auth/provider.go)
- [`internal/store/checks.go`](../../internal/store/checks.go)
