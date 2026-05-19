import type { ModelState, RecentRun } from '@/types'

/** Vuetify chip colors used by last-check status cells. */
export type CheckColor = 'secondary' | 'success' | 'error'

/** Model inventory row enriched with table-only fields. */
export type ModelInventoryRow = ModelState & {
  status_label: string
  last_check_label: string
  last_check_color: CheckColor
  last_check_at: number | null
}

/** Builds the latest known run map for model inventory status chips. */
export function lastRunByModel(runs: RecentRun[] = []) {
  const map = new Map<string, RecentRun>()
  for (const run of runs) {
    const previous = map.get(run.model_id)
    if (!previous || isRunNewer(run, previous)) {
      map.set(run.model_id, run)
    }
  }
  return map
}

/** Enriches raw model state with table-only display fields. */
export function modelInventoryRows(models: ModelState[], runs: Map<string, RecentRun>): ModelInventoryRow[] {
  return models.map(model => {
    const lastRun = runs.get(model.model_id)
    return {
      ...model,
      status_label: statusLabel(model),
      last_check_label: checkLabel(lastRun),
      last_check_color: checkColor(lastRun),
      last_check_at: timestampFor(lastRun?.started_at)
    }
  })
}

/** Compares nullable timestamps for Vuetify table sorting. */
export function compareNullableNumbers(a: number | null, b: number | null) {
  if (a === b) return 0
  if (a === null) return -1
  if (b === null) return 1
  return a - b
}

/** Formats API timestamps for compact table display. */
export function formatTime(value?: string) {
  if (!value) return 'n/a'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(value))
}

/** Chooses the Vuetify color for a model status chip. */
export function statusColor(model: ModelState) {
  if (model.capability === 'unknown') return 'warning'
  if (model.excluded || model.capability === 'skip') return 'secondary'
  return model.status === 'active' ? 'success' : 'error'
}

/** Parses API timestamps into sortable millisecond values. */
function timestampFor(value?: string) {
  if (!value) return null
  const timestamp = Date.parse(value)
  return Number.isNaN(timestamp) ? null : timestamp
}

/** Reports whether one recent run is newer than another. */
function isRunNewer(run: RecentRun, previous: RecentRun) {
  const runTimestamp = timestampFor(run.started_at)
  const previousTimestamp = timestampFor(previous.started_at)
  if (previousTimestamp === null) return runTimestamp !== null
  if (runTimestamp === null) return false
  return runTimestamp > previousTimestamp
}

/** Builds the table-facing status label for a model. */
function statusLabel(model: ModelState) {
  return model.excluded ? 'excluded' : model.status
}

/** Chooses the Vuetify color for the latest run chip. */
function checkColor(run?: RecentRun): CheckColor {
  if (!run) return 'secondary'
  return run.ok ? 'success' : 'error'
}

/** Builds the latest run label shown in the table. */
function checkLabel(run?: RecentRun) {
  if (!run) return 'no data'
  return run.ok ? `ok · ${Math.round(run.latency_ms)} ms` : `error · ${run.status_code || '?'}`
}
