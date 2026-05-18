# Model Snapshot Task

## Purpose

`RefreshModels` snapshots the current model inventory, classifies model
capabilities, updates lifecycle state, reloads the runnable model plan, and emits
model lifecycle alerts.

## Schedule

- Config key: `schedules.model_snapshot`
- Default interval: `5m`
- Startup behavior: runs once before the first scheduled model run, then repeats
  only after the first configured interval has elapsed.

## Inputs

- `target.base_url` and authentication settings for `GET /v1/models`.
- `models.max_concurrency`, defaulting to `4`, to bound parallel capability probes.
- `models.absence_alert_after`, defaulting to `24h`, for missing and returned alerts.
- `tests.embedding_fixture.path` and `tests.embedding_fixture.max_bytes` for
  embedding capability probes.
- `smtp.*` settings when email alerts are enabled.

## Execution

The task first calls `/v1/models` and extracts model IDs. Each model is classified
by probing chat first, then embeddings:

- Chat probe: `POST /v1/chat/completions` with prompt `Reply with ok.`
- Embedding probe: `POST /v1/embeddings` with the configured embedding fixture or a fallback probe string.

If a chat probe succeeds, the model is classified as `chat`. If chat fails and the
embedding probe returns a non-empty vector, the model is classified as `embedding`.
If both probes fail with transient signals such as rate limits, timeouts, or 5xx
responses, the capability is marked `unknown`; the last known runnable capability
is preserved when available. Otherwise the model is marked `skip`.

The resulting observation is persisted, current model state is updated, lifecycle
events are derived, and the in-memory model plan is replaced with runnable
`chat` and `embedding` models.

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
- `removed`
- `capability_changed`
- `capability_probe`
- `alert_sent`
- `alert_failed`

## Alerts

Model lifecycle email alerts are deduplicated by alert key:

- `first_seen`: sent when a model appears for the first time.
- `missing`: sent when a model has been absent longer than `models.absence_alert_after`.
- `returned`: sent when a model returns after a long absence.

Alert attempts are recorded even when SMTP delivery fails, and failures also create
model events.

## Failure Behavior

If `/v1/models` fails, the task returns an error and does not update the inventory.
Individual capability probe failures are captured in model event details where
possible. If persistence fails while processing the observation, the task returns
an error and the scheduler logs it.

## Related Code

- [`internal/monitor/inventory.go`](../../internal/monitor/inventory.go)
- [`internal/monitor/capabilities.go`](../../internal/monitor/capabilities.go)
- [`internal/monitor/rules.go`](../../internal/monitor/rules.go)
- [`internal/monitor/events.go`](../../internal/monitor/events.go)
- [`internal/monitor/alerts.go`](../../internal/monitor/alerts.go)
- [`internal/store/models.go`](../../internal/store/models.go)
