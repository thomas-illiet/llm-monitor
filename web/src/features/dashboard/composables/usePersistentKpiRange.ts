import { shallowRef, watch } from 'vue'
import type { KpiRangePreset, KpiRangeValue } from '@/types'

const STORAGE_KEY = 'llm-monitor-kpi-range'
const DEFAULT_KPI_RANGE: KpiRangeValue = '24h'

/** KPI range options shown in the dashboard segmented control. */
export const KPI_RANGE_PRESETS = [
  { label: 'Last day', value: '24h' },
  { label: 'Last 7 days', value: '168h' },
  { label: 'Last month', value: '720h' },
  { label: 'Last year', value: '8760h' }
] as const satisfies readonly KpiRangePreset[]

const KPI_RANGE_VALUES = new Set<KpiRangeValue>(
  KPI_RANGE_PRESETS.map(preset => preset.value)
)

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
