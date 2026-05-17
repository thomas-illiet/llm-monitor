import { onMounted, onUnmounted, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import type { DashboardData, KpiRangeValue } from '@/types'

interface UseDashboardDataOptions {
  intervalMs?: number
  kpiRange?: MaybeRefOrGetter<KpiRangeValue>
  minLoadMs?: number
}

/** Loads and refreshes the dashboard aggregate API payload. */
export function useDashboardData(options: UseDashboardDataOptions = {}) {
  const data = shallowRef<DashboardData | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const intervalMs = options.intervalMs ?? 30_000
  const minLoadMs = options.minLoadMs ?? 600
  let timer: number | undefined

  /** Builds the dashboard API path for the active KPI range. */
  function dashboardPath() {
    const params = new URLSearchParams()
    const range = options.kpiRange ? toValue(options.kpiRange) : ''
    if (range) params.set('range', range)
    const query = params.toString()
    return query ? `/api/dashboard?${query}` : '/api/dashboard'
  }

  /** Refreshes dashboard data while keeping the loading state visually stable. */
  async function refresh() {
    loading.value = true
    error.value = null
    const minWait = new Promise<void>(r => setTimeout(r, minLoadMs))
    try {
      const response = await fetch(dashboardPath(), {
        headers: { Accept: 'application/json' }
      })
      if (!response.ok) {
        throw new Error(`Dashboard API returned ${response.status}`)
      }
      data.value = await response.json() as DashboardData
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Unable to load dashboard'
    } finally {
      await minWait
      loading.value = false
    }
  }

  onMounted(() => {
    void refresh()
    timer = window.setInterval(() => { void refresh() }, intervalMs)
  })

  watch(() => options.kpiRange ? toValue(options.kpiRange) : undefined, () => {
    void refresh()
  })

  onUnmounted(() => {
    if (timer !== undefined) window.clearInterval(timer)
  })

  return { data, loading, error, refresh }
}
