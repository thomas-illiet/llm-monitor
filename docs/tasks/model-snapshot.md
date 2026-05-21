# Model Snapshot Task

## Purpose

`monitor.model_snapshot` snapshots the current model inventory, classifies model
capabilities, updates lifecycle state, and emits model lifecycle alerts.

## Schedule

- Config key: `schedules.model_snapshot`
- Default interval: `5m`
- Startup behavior: enqueued at scheduler startup, then repeats on the configured
  interval.
- Payload: empty JSON payload.

## Inputs

- `target.base_url`, `target.endpoints.models`, and authentication settings for
  model inventory requests.
- `target.endpoints.chat` and `target.endpoints.embeddings` for capability probes.
- `models.max_concurrency`, defaulting to `4`, to bound parallel capability probes.
- `models.absence_alert_after`, defaulting to `24h`, for inactive and returned alerts.
- `tests.embedding_fixture.path` and `tests.embedding_fixture.max_bytes` for
  embedding capability probes.
- `smtp.*` settings when email alerts are enabled.

## Execution

The handler from `internal/schedule/tasks/models/model_snapshot.go` first calls
the configured model inventory endpoint and extracts model IDs. Each model is
classified by probing chat first, then embeddings:

- Chat probe: `POST target.endpoints.chat` with prompt `Reply with ok.`
- Embedding probe: `POST target.endpoints.embeddings` with the configured embedding fixture or a fallback probe string.

If a chat probe succeeds, the model is classified as `chat`. If chat fails and the
embedding probe returns a non-empty vector, the model is classified as `embedding`.
If both probes fail with transient signals such as rate limits, timeouts, or 5xx
responses, the capability is marked `unknown`; the last known runnable capability
is preserved when available. Otherwise the model is marked `skip`.

The resulting observation is persisted, current model state is updated, and
lifecycle events are derived. The Asynq scheduler reads active runnable model
state from PostgreSQL on each sync to add, remove, or retime per-model runs.

## Stored Output

The task writes or updates:

- `model_snapshots`
- `model_snapshot_items`
- `model_states`
- `model_events`
- `email_alerts`, when lifecycle alert delivery is attempted.

Lifecycle events include:

- `added`
- `returned`
- `inactive`
- `capability_changed`
- `capability_probe`
- `alert_sent`
- `alert_failed`

## Alerts

Model lifecycle email alerts are deduplicated by alert key:

- `first_seen`: sent when a model appears for the first time.
- `inactive`: sent when a model has been absent or unavailable longer than `models.absence_alert_after`.
- `returned`: sent when a model returns after a long absence.

Alert attempts are recorded even when SMTP delivery fails, and failures also create
model events.

## Failure Behavior

If the configured model inventory endpoint fails after configured retries, currently runnable models are marked inactive and the task returns the upstream error.
Individual capability probe failures are captured in model event details where
possible. If persistence fails while processing the observation, the task returns
an error so the worker records the failure.

## Related Code

- [`internal/schedule/tasks/models/model_snapshot.go`](../../internal/schedule/tasks/models/model_snapshot.go)
- [`internal/schedule/tasks/models/capabilities.go`](../../internal/schedule/tasks/models/capabilities.go)
- [`internal/schedule/tasks/models/rules.go`](../../internal/schedule/tasks/models/rules.go)
- [`internal/schedule/tasks/models/events.go`](../../internal/schedule/tasks/models/events.go)
- [`internal/schedule/tasks/models/alerts.go`](../../internal/schedule/tasks/models/alerts.go)
- [`internal/schedule/runner`](../../internal/schedule/runner)
- [`internal/store/models.go`](../../internal/store/models.go)
