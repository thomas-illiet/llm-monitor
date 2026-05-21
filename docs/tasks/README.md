# Scheduled Tasks

The backend registers monitor tasks through `internal/schedule/tasks` in the
`internal/schedule/runner` registry, then runs them through Asynq workers.
Concrete task handlers are grouped by category under `checks`, `models`, and
`retention`. Task names are stable across scheduled and manually enqueued work.

Each task below has its own self-contained note with its schedule, payload,
inputs, outputs, failure behavior, and related code.

## Source Layout

- `internal/schedule/runner`: generic task primitives and local recurring runner.
- `internal/schedule/queue`: Asynq constructors, worker mux, scheduler provider, and manual queue inspection.
- `internal/schedule/tasks`: registry and task wiring.
- `internal/schedule/tasks/checks`: HTTP and auth health checks.
- `internal/schedule/tasks/models`: model inventory, capability detection, scheduled probes, events, and alerts.
- `internal/schedule/tasks/retention`: history pruning.
- `internal/schedule/tasks/shared`: task names, shared dependencies, repository/client contracts, and queued payloads.

| Task | Stable name | Trigger | Default interval |
| --- | --- | --- | --- |
| [HTTP check](http-check.md) | `monitor.http_check` | Starts immediately, then repeats | `schedules.http_check`, default `30s` |
| [Auth check](auth-check.md) | `monitor.auth_check` | Starts immediately, then repeats | `schedules.auth_check`, default `60s` |
| [Model snapshot](model-snapshot.md) | `monitor.model_snapshot` | Enqueued at scheduler startup, then repeats | `schedules.model_snapshot`, default `5m` |
| [Model runs](model-runs.md) | `monitor.model_run` | One dynamic schedule per runnable model | `schedules.model_runs`, default `15m`, with optional overrides |
| [History retention](history-retention.md) | `monitor.history_retention` | Starts immediately when enabled, then repeats | fixed `24h` loop, controlled by `retention.history` |

The scheduler process owns recurring entries and refreshes them from PostgreSQL.
Worker processes own execution. Each invocation gets a `TaskContext` with
`task_name`, `run_id`, `attempt`, `scheduled_at`, and an optional JSON payload.
Manual dashboard checks enqueue the same task types with temporary result
retention for spinner polling.
