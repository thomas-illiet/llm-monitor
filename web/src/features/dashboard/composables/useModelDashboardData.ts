import { onMounted, onUnmounted, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import type { KpiRangeValue, ModelDashboardData } from '@/types'

interface UseModelDashboardDataOptions {
  intervalMs?: number
  kpiRange?: MaybeRefOrGetter<KpiRangeValue>
  modelId: MaybeRefOrGetter<string | null>
}

/** Loads and refreshes model-scoped dashboard telemetry. */
export function useModelDashboardData(options: UseModelDashboardDataOptions) {
  const data = shallowRef<ModelDashboardData | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const intervalMs = options.intervalMs ?? 30_000
  let timer: number | undefined
  let controller: AbortController | null = null

  /** Builds the model dashboard API path for the active model and KPI range. */
  function modelDashboardPath() {
    const modelId = toValue(options.modelId)
    if (!modelId) return null
    const params = new URLSearchParams({ model_id: modelId })
    const range = options.kpiRange ? toValue(options.kpiRange) : ''
    if (range) params.set('range', range)
    return `/api/model-dashboard?${params}`
  }

  /** Refreshes the selected model dashboard payload. */
  async function refresh() {
    const path = modelDashboardPath()
    if (!path) {
      controller?.abort()
      data.value = null
      loading.value = false
      error.value = null
      return
    }

    controller?.abort()
    controller = new AbortController()
    const activeController = controller
    loading.value = true
    error.value = null
    try {
      const response = await fetch(path, {
        headers: { Accept: 'application/json' },
        signal: activeController.signal
      })
      if (!response.ok) {
        throw new Error(response.status === 404 ? 'Model not found' : `Model dashboard API returned ${response.status}`)
      }
      data.value = await response.json() as ModelDashboardData
    } catch (err) {
      if (!activeController.signal.aborted) {
        error.value = err instanceof Error ? err.message : 'Unable to load model dashboard'
      }
    } finally {
      if (!activeController.signal.aborted) {
        loading.value = false
      }
    }
  }

  watch(
    () => [toValue(options.modelId), options.kpiRange ? toValue(options.kpiRange) : undefined] as const,
    () => {
      void refresh()
    },
    { immediate: true }
  )

  onMounted(() => {
    timer = window.setInterval(() => { void refresh() }, intervalMs)
  })

  onUnmounted(() => {
    controller?.abort()
    if (timer !== undefined) window.clearInterval(timer)
  })

  return { data, loading, error, refresh }
}
