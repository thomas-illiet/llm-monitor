/** Aggregate payload returned by `/api/dashboard`. */
export interface DashboardData {
  generated_at: string
  status: StatusSummary
  kpis: KpiSummary
  slo: SloThresholds
  charts: ConfiguredChart[]
  model_status_history: ConfiguredChart
  providers: ProviderStatus[]
  models: ModelState[]
  events: ModelEvent[]
  runs: RecentRun[]
  alerts: RecentAlert[]
  auth?: CheckRecord
  http?: CheckRecord
  config: RuntimeConfig
}

/** Model-scoped payload returned by `/api/providers/{provider_id}/dashboard`. */
export interface ModelDashboardData {
  generated_at: string
  model: ModelState
  kpis: KpiSummary
  slo: SloThresholds
  charts: ConfiguredChart[]
  runs: RecentRun[]
}

/** Supported preset KPI time-window values shown by the dashboard UI. */
export type KpiRangeValue = '12h' | '24h' | '168h' | '720h' | '8760h'

/** One selectable KPI range preset. */
export interface KpiRangePreset {
  label: string
  value: KpiRangeValue
}

/** Non-secret runtime settings returned with dashboard data. */
interface RuntimeConfig {
  retention: RetentionRuntimeConfig
  site_name: string
  site_url?: string
  providers: ProviderRuntimeConfig[]
}

export interface ProviderRuntimeConfig {
  id: string
  name: string
}

export interface ProviderStatus {
  id: string
  name: string
  auth?: CheckRecord
  http?: CheckRecord
}

/** Effective history retention settings. */
interface RetentionRuntimeConfig {
  history_seconds: number
}

/** Compact service health summary. */
export interface StatusSummary {
  ok: boolean
  generated_at: string
  active_models: number
  inactive_models: number
  missing_models: number
  skipped_models: number
  auth_ok: boolean
  http_ok: boolean
}

/** Aggregated latency, reliability, throughput, and token counters. */
export interface KpiSummary {
  total_runs: number
  success_rate: number
  latency_p50_ms: number
  latency_p95_ms: number
  latency_p99_ms: number
  request_latency_p50_ms: number
  request_latency_p90_ms: number
  request_latency_p95_ms: number
  request_latency_p99_ms: number
  ttft_p50_ms: number
  ttft_p90_ms: number
  ttft_p95_ms: number
  ttft_p99_ms: number
  itl_p50_ms: number
  itl_p90_ms: number
  itl_p95_ms: number
  itl_p99_ms: number
  tpot_p50_ms: number
  tpot_p90_ms: number
  tpot_p95_ms: number
  tpot_p99_ms: number
  output_tokens_per_second: number
  slo_violation_count: number
  degraded_models: number
  error_count: number
  input_tokens: number
  output_tokens: number
}

/** Dashboard SLO thresholds used to label degraded performance. */
export interface SloThresholds {
  ttft_p99_ms: number
  itl_p99_ms: number
  request_latency_p99_ms: number
}

/** One configured chart returned with ready-to-render series. */
export interface ConfiguredChart {
  id: string
  title: string
  type: 'line' | 'bar' | 'stacked-bar'
  metric: string
  labels: string[]
  datasets: ChartDataset[]
  error?: string
}

/** One chart series with numeric samples. */
interface ChartDataset {
  label: string
  data: Array<number | null>
}

/** Current state of one model in the monitored inventory. */
export interface ModelState {
  provider_id: string
  model_id: string
  model_key: string
  capability: 'chat' | 'embedding' | 'skip' | string
  excluded: boolean
  status: 'active' | 'inactive' | string
  first_seen_at: string
  last_seen_at: string
  next_check_at?: string
  missing_since?: string
  skip_reason?: string
  last_probe_at?: string
}

/** Provider metadata details returned for one model. */
export interface ModelTechnicalDetailsData {
  generated_at: string
  model: ModelState
  provider_metadata: Record<string, unknown>
}

/** One model lifecycle or diagnostic timeline event. */
export interface ModelEvent {
  id: number
  provider_id: string
  model_id: string
  model_key: string
  event_type: 'added' | 'removed' | 'returned' | 'capability_probe' | 'scheduled_run' | 'skipped' | string
  source: string
  severity: 'info' | 'warning' | 'error' | string
  status: 'ok' | 'error' | 'skipped' | 'inactive' | string
  capability: string
  observed_at: string
  title: string
  message: string
  changed: boolean
  details?: Record<string, unknown>
}

/** Paginated response returned by `/api/providers/{provider_id}/models/{model_key}/events`. */
export interface ModelEventsResponse {
  provider_id?: string
  model_id: string
  events: ModelEvent[]
  total: number
  limit: number
  offset: number
  filters: ModelEventFilterOptions
}

/** Available filters for a model event timeline. */
export interface ModelEventFilterOptions {
  statuses: string[]
  sources: string[]
  event_types: string[]
}

/** Shared scheduled probe fields shown in dashboard run timelines. */
interface BaseRecentRun {
  provider_id: string
  model_id: string
  model_key: string
  started_at: string
  ok: boolean
  status_code: number
  latency_ms: number
  input_tokens?: number
  total_tokens?: number
  error?: string
}

/** Recent chat probe result shown in the dashboard. */
interface ChatRecentRun extends BaseRecentRun {
  capability: 'chat'
  prompt_id?: string
  output_tokens?: number
}

/** Recent embedding probe result shown in the dashboard. */
export interface EmbeddingRecentRun extends BaseRecentRun {
  capability: 'embedding'
  fixture_path?: string
  fixture_bytes?: number
  vector_dimensions?: number
  output_tokens?: never
  prompt_id?: never
}

/** Recent scheduled probe result shown in the dashboard. */
export type RecentRun = ChatRecentRun | EmbeddingRecentRun

/** Recent lifecycle alert email shown in the dashboard. */
interface RecentAlert {
  provider_id: string
  model_id: string
  type: string
  sent_at: string
  subject: string
  recipients: string[]
  error?: string
}

/** Latest HTTP or auth availability check. */
export interface CheckRecord {
  at: string
  ok: boolean
  status_code: number
  latency_ms: number
  expires_at?: string
  error: string
}

/** One manual check task accepted by the queue. */
interface ManualCheckJob {
  id: string
  queue: string
  type: string
  provider_id?: string
  model_id?: string
  state: string
}

/** Response returned after requesting manual checks. */
export interface ManualCheckRunResponse {
  jobs: ManualCheckJob[]
}

/** Latest queue state for one manual check task. */
interface ManualCheckJobStatus {
  id: string
  queue: string
  type?: string
  provider_id?: string
  model_id?: string
  state: string
  error?: string
  completed_at?: string
}

/** Response returned while polling manual check tasks. */
export interface ManualCheckJobsResponse {
  jobs: ManualCheckJobStatus[]
}
