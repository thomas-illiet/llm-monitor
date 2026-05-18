# Scheduled Tasks

The backend registers monitor tasks through `internal/schedule/tasks` in the
`internal/schedule/runner` registry, then runs them with the in-process
`LocalScheduler`. Concrete task handlers are grouped by category under
`checks`, `models`, and `retention`. Task names are stable so the same business
handlers can later be executed by a queue-backed or distributed worker.

Each task below has its own self-contained note with its schedule, payload,
inputs, outputs, failure behavior, and related code.

## Source Layout

- `internal/schedule/runner`: generic task primitives and local recurring runner.
- `internal/schedule/tasks`: registry and local schedule wiring.
- `internal/schedule/tasks/checks`: HTTP and auth health checks.
- `internal/schedule/tasks/models`: model inventory, capability detection, scheduled probes, events, and alerts.
- `internal/schedule/tasks/retention`: history pruning.
- `internal/schedule/tasks/shared`: task names, shared dependencies, repository/client contracts, and `ModelPlanStore`.

| Task | Stable name | Trigger | Default interval |
| --- | --- | --- | --- |
| [HTTP check](http-check.md) | `monitor.http_check` | Starts immediately, then repeats | `schedules.http_check`, default `30s` |
| [Auth check](auth-check.md) | `monitor.auth_check` | Starts immediately, then repeats | `schedules.auth_check`, default `60s` |
| [Model snapshot](model-snapshot.md) | `monitor.model_snapshot` | Runs before the first model run, then repeats after the first interval | `schedules.model_snapshot`, default `5m` |
| [Model runs](model-runs.md) | `monitor.model_runs` | Starts after the initial model snapshot, then repeats | `schedules.model_runs`, default `15m` |
| [History retention](history-retention.md) | `monitor.history_retention` | Starts immediately when enabled, then repeats | fixed `24h` loop, controlled by `retention.history` |

The local scheduler lifetime is bound to the server context. Each invocation gets
a `TaskContext` with `task_name`, `run_id`, `attempt`, `scheduled_at`, and an
optional JSON payload. Current monitor tasks use empty payloads. If a task returns
an error, the local loop logs `scheduled task failed` and continues on the next
tick.
