# Model Runs Task

## Purpose

`monitor.model_runs` executes recurring performance and availability probes
against the latest runnable model plan produced by the model snapshot task.

## Schedule

- Config key: `schedules.model_runs`
- Default interval: `15m`
- Startup behavior: runs immediately after the initial model snapshot has loaded
  the model plan, then repeats on the configured interval.
- Payload: empty JSON payload.

## Inputs

- Current shared `ModelPlanStore` populated by `monitor.model_snapshot`.
- `models.max_concurrency`, defaulting to `4`, to bound parallel model probes.
- `tests.chat_prompts`: prompt IDs, prompt text, max token limits, and temperatures
  for chat models.
- `tests.embedding_fixture.path` and `tests.embedding_fixture.max_bytes` for
  embedding models.

## Execution

The handler from `internal/schedule/tasks/models/model_runs.go` reads the shared
model plan. If the plan is empty, the task exits successfully without work.
Otherwise, it runs model probes concurrently up to `models.max_concurrency`.

For models classified as `chat`, the task runs every configured chat prompt with a
non-empty `id` and `prompt`. It uses streaming chat completions so it can capture
GuideLLM-style latency metrics such as time to first token and inter-token latency.

For models classified as `embedding`, the task loads the embedding fixture,
truncates it to `tests.embedding_fixture.max_bytes`, and sends it to the embedding
endpoint. If no fixture path is configured, a small built-in fixture string is used.

## Stored Output

Chat probe results are inserted into `chat_runs`:

- `model_id`
- `prompt_id`
- `started_at`
- `ok`
- `status_code`
- `latency_ms`
- `ttft_ms`
- `itl_ms`
- `tpot_ms`
- `request_latency_ms`
- `input_tokens`
- `output_tokens`
- `total_tokens`
- `output_tokens_per_second`
- `error`

Embedding probe results are inserted into `embedding_runs`:

- `model_id`
- `fixture_path`
- `fixture_bytes`
- `started_at`
- `ok`
- `status_code`
- `latency_ms`
- `input_tokens`
- `total_tokens`
- `vector_dimensions`
- `error`

Each successful or failed probe also creates a `scheduled_run` model event when
the run result can be recorded.

## Failure Behavior

Invalid chat prompts are skipped. If a chat model has no valid prompts, the task
records a warning model event and continues.

If the embedding fixture is empty or unreadable, the task records a skipped model
event and does not call the embedding endpoint.

Probe errors are stored with the run result and reflected in the corresponding
model event. Storage errors are joined and returned after all scheduled model
probes finish, allowing the local scheduler to log the failure while preserving
as many results as possible.

## Related Code

- [`internal/schedule/tasks/models/model_runs.go`](../../internal/schedule/tasks/models/model_runs.go)
- [`internal/schedule/tasks/models/fixture.go`](../../internal/schedule/tasks/models/fixture.go)
- [`internal/schedule/tasks/models/events.go`](../../internal/schedule/tasks/models/events.go)
- [`internal/schedule/runner`](../../internal/schedule/runner)
- [`internal/llm/chat.go`](../../internal/llm/chat.go)
- [`internal/llm/embedding.go`](../../internal/llm/embedding.go)
- [`internal/store/runs.go`](../../internal/store/runs.go)
