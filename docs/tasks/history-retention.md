# History Retention Task

## Purpose

`monitor.history_retention` prunes old persisted history while preserving the
current model state needed by the dashboard and alerting logic.

## Schedule

- Config key: `retention.history`
- Default retention window: `90d` when the field is omitted.
- Disable behavior: set `retention.history` to `0s`.
- Loop interval: fixed `24h`.
- Startup behavior: when retention is enabled, runs once immediately, then repeats
  every 24 hours.
- Payload: empty JSON payload.

## Inputs

- `retention.history`: duration to keep historical rows. Values use the same
  duration parser as other config fields and support day values such as `30d` or
  `90d`.

## Execution

The handler from `internal/schedule/tasks/retention/history_retention.go`
computes a cutoff as:

```text
now_utc - retention.history
```

It then deletes historical rows older than the cutoff inside one database
transaction.

## Pruned Tables

The retention task deletes old rows from:

- `model_events`
- `http_checks`
- `auth_checks`
- `chat_runs`
- `embedding_runs`
- `email_alerts`
- `model_snapshots`

Current `model_states` are not pruned. Inactive-model alert records are preserved
while the model is still inactive, even when their `sent_at` timestamp is older
than the cutoff.

## Failure Behavior

If retention is disabled or storage is unavailable, the task exits without work.
Database errors abort the transaction and are returned to the local scheduler,
which logs them and continues on the next tick.

## Related Code

- [`internal/schedule/tasks/retention/history_retention.go`](../../internal/schedule/tasks/retention/history_retention.go)
- [`internal/schedule/runner`](../../internal/schedule/runner)
- [`internal/store/retention.go`](../../internal/store/retention.go)
- [`internal/config/config.go`](../../internal/config/config.go)
