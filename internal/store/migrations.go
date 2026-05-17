package store

const migrationSQL = `
CREATE TABLE IF NOT EXISTS model_snapshots (
  id BIGSERIAL PRIMARY KEY,
  observed_at TIMESTAMPTZ NOT NULL,
  raw JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS model_snapshot_items (
  snapshot_id BIGINT NOT NULL REFERENCES model_snapshots(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL,
  capability TEXT NOT NULL,
  excluded BOOLEAN NOT NULL DEFAULT FALSE,
  skip_reason TEXT NOT NULL DEFAULT '',
  probe_details JSONB NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (snapshot_id, model_id)
);

CREATE TABLE IF NOT EXISTS model_states (
  model_id TEXT PRIMARY KEY,
  capability TEXT NOT NULL,
  excluded BOOLEAN NOT NULL DEFAULT FALSE,
  status TEXT NOT NULL,
  first_seen_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  missing_since TIMESTAMPTZ,
  skip_reason TEXT NOT NULL DEFAULT '',
  last_probe_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS model_events (
  id BIGSERIAL PRIMARY KEY,
  model_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  source TEXT NOT NULL,
  severity TEXT NOT NULL,
  status TEXT NOT NULL,
  capability TEXT NOT NULL,
  observed_at TIMESTAMPTZ NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  details JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS model_events_observed_at_idx ON model_events(observed_at DESC);
CREATE INDEX IF NOT EXISTS model_events_model_observed_idx ON model_events(model_id, observed_at DESC);

CREATE TABLE IF NOT EXISTS http_checks (
  id BIGSERIAL PRIMARY KEY,
  checked_at TIMESTAMPTZ NOT NULL,
  ok BOOLEAN NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  latency_ms DOUBLE PRECISION NOT NULL,
  error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS http_checks_checked_at_idx ON http_checks(checked_at DESC);

CREATE TABLE IF NOT EXISTS auth_checks (
  id BIGSERIAL PRIMARY KEY,
  checked_at TIMESTAMPTZ NOT NULL,
  ok BOOLEAN NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  latency_ms DOUBLE PRECISION NOT NULL,
  expires_at TIMESTAMPTZ,
  error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS auth_checks_checked_at_idx ON auth_checks(checked_at DESC);

CREATE TABLE IF NOT EXISTS chat_runs (
  id BIGSERIAL PRIMARY KEY,
  model_id TEXT NOT NULL,
  prompt_id TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  ok BOOLEAN NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  latency_ms DOUBLE PRECISION NOT NULL,
  ttft_ms DOUBLE PRECISION,
  itl_ms DOUBLE PRECISION,
  tpot_ms DOUBLE PRECISION,
  request_latency_ms DOUBLE PRECISION,
  input_tokens INTEGER,
  output_tokens INTEGER,
  total_tokens INTEGER,
  output_tokens_per_second DOUBLE PRECISION,
  error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS chat_runs_started_at_idx ON chat_runs(started_at DESC);
CREATE INDEX IF NOT EXISTS chat_runs_model_idx ON chat_runs(model_id, started_at DESC);

CREATE TABLE IF NOT EXISTS embedding_runs (
  id BIGSERIAL PRIMARY KEY,
  model_id TEXT NOT NULL,
  fixture_path TEXT NOT NULL,
  fixture_bytes INTEGER NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ NOT NULL,
  ok BOOLEAN NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  latency_ms DOUBLE PRECISION NOT NULL,
  input_tokens INTEGER,
  total_tokens INTEGER,
  vector_dimensions INTEGER,
  error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS embedding_runs_started_at_idx ON embedding_runs(started_at DESC);
CREATE INDEX IF NOT EXISTS embedding_runs_model_idx ON embedding_runs(model_id, started_at DESC);

CREATE TABLE IF NOT EXISTS email_alerts (
  id BIGSERIAL PRIMARY KEY,
  alert_key TEXT NOT NULL UNIQUE,
  model_id TEXT NOT NULL,
  alert_type TEXT NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL,
  subject TEXT NOT NULL,
  recipients TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS email_alerts_sent_at_idx ON email_alerts(sent_at DESC);
`
