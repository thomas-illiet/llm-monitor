# History Retention Task

## Purpose

`RunHistoryRetention` prunes old persisted history while preserving the current
model state needed by the dashboard and alerting logic.

## Schedule

- Config key: `retention.history`
- Default retention window: `90d` when the field is omitted.
- Disable behavior: set `retention.history` to `0s`.
- Loop interval: fixed `24h`.
- Startup behavior: when retention is enabled, runs once immediately, then repeats
  every 24 hours.

## Inputs

- `retention.history`: duration to keep historical rows. Values use the same
  duration parser as other config fields and support day values such as `30d` or
  `90d`.

## Execution

The task computes a cutoff as:

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

Current `model_states` are not pruned. Missing-model alert records are preserved
while the model is still actively missing, even when their `sent_at` timestamp is
older than the cutoff.

## Failure Behavior

If retention is disabled or storage is unavailable, the task exits without work.
Database errors abort the transaction, return an error, and are logged by the
scheduler loop.

## Related Code

- [`internal/monitor/retention.go`](../../internal/monitor/retention.go)
- [`internal/store/retention.go`](../../internal/store/retention.go)
- [`internal/config/config.go`](../../internal/config/config.go)
