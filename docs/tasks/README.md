# Scheduled Tasks

The backend scheduler starts independent recurring tasks from `internal/monitor`.
Each task below has its own self-contained note with its schedule, inputs, outputs,
failure behavior, and related code.

| Task | Function | Trigger | Default interval |
| --- | --- | --- | --- |
| [HTTP check](http-check.md) | `RunHTTPCheck` | Starts immediately, then repeats | `schedules.http_check`, default `30s` |
| [Auth check](auth-check.md) | `RunAuthCheck` | Starts immediately, then repeats | `schedules.auth_check`, default `60s` |
| [Model snapshot](model-snapshot.md) | `RefreshModels` | Runs before the first model run, then repeats after the first interval | `schedules.model_snapshot`, default `5m` |
| [Model runs](model-runs.md) | `RunModelTests` | Starts after the initial model snapshot, then repeats | `schedules.model_runs`, default `15m` |
| [History retention](history-retention.md) | `RunHistoryRetention` | Starts immediately when enabled, then repeats | fixed `24h` loop, controlled by `retention.history` |

The scheduler lifetime is bound to the server context. If a task returns an error,
the loop logs `scheduled task failed` and continues on the next tick.
