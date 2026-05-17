import { shallowRef, watch } from 'vue'
import type { KpiRangePreset, KpiRangeValue } from '@/types'

const STORAGE_KEY = 'llm-monitor-kpi-range'
const DEFAULT_KPI_RANGE: KpiRangeValue = '24h'

/** KPI range options shown in the dashboard dropdown. */
export const KPI_RANGE_PRESETS = [
  { label: 'Last 1 hour', value: '1h' },
  { label: 'Last 12 hours', value: '12h' },
  { label: 'Last day', value: '24h' },
  { label: 'Last 7 days', value: '168h' },
  { label: 'Last month', value: '720h' },
  { label: 'Last year', value: '8760h' }
] as const satisfies readonly KpiRangePreset[]

const KPI_RANGE_VALUES = new Set<KpiRangeValue>(
  KPI_RANGE_PRESETS.map(preset => preset.value)
)

const KPI_RANGE_SECONDS: Record<KpiRangeValue, number> = {
  '1h': 60 * 60,
  '12h': 12 * 60 * 60,
  '24h': 24 * 60 * 60,
  '168h': 7 * 24 * 60 * 60,
  '720h': 30 * 24 * 60 * 60,
  '8760h': 365 * 24 * 60 * 60
}

/** Validates stored string values before using them as KPI ranges. */
function isKpiRangeValue(value: string | null): value is KpiRangeValue {
  return value !== null && KPI_RANGE_VALUES.has(value as KpiRangeValue)
}

/** Reads the persisted KPI range or falls back to the default window. */
function readStoredKpiRange(): KpiRangeValue {
  const stored = window.localStorage.getItem(STORAGE_KEY)
  return isKpiRangeValue(stored) ? stored : DEFAULT_KPI_RANGE
}

/** Persists the selected KPI range in local storage. */
export function usePersistentKpiRange() {
  const selectedKpiRange = shallowRef<KpiRangeValue>(readStoredKpiRange())

  watch(selectedKpiRange, (range) => {
    window.localStorage.setItem(STORAGE_KEY, range)
  }, { immediate: true })

  return {
    selectedKpiRange
  }
}

/** Filters KPI presets to the configured retention window when retention is enabled. */
export function filterKpiRangePresets(presets: readonly KpiRangePreset[], historySeconds: number) {
  if (historySeconds <= 0) return presets
  const filtered = presets.filter(preset => KPI_RANGE_SECONDS[preset.value] <= historySeconds)
  return filtered.length > 0 ? filtered : presets.slice(0, 1)
}

/** Keeps the selected KPI range inside the currently available preset list. */
export function clampKpiRangeValue(value: KpiRangeValue, presets: readonly KpiRangePreset[]): KpiRangeValue {
  if (presets.some(preset => preset.value === value)) return value
  return presets[presets.length - 1]?.value ?? DEFAULT_KPI_RANGE
}
