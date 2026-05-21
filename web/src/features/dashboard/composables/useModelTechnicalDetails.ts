import { onUnmounted, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import type { ModelTechnicalDetailsData } from '@/types'

interface UseModelTechnicalDetailsOptions {
  modelId: MaybeRefOrGetter<string | null>
  open: MaybeRefOrGetter<boolean>
}

/** Loads provider metadata for the selected model on demand. */
export function useModelTechnicalDetails(options: UseModelTechnicalDetailsOptions) {
  const data = shallowRef<ModelTechnicalDetailsData | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  let controller: AbortController | null = null

  function detailsPath() {
    const modelId = toValue(options.modelId)
    if (!toValue(options.open) || !modelId) return null
    return `/api/models/${encodeURIComponent(modelId)}/details`
  }

  async function refresh() {
    const path = detailsPath()
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
        throw new Error(response.status === 404 ? 'Model not found' : `Model details API returned ${response.status}`)
      }
      data.value = await response.json() as ModelTechnicalDetailsData
    } catch (err) {
      if (!activeController.signal.aborted) {
        error.value = err instanceof Error ? err.message : 'Unable to load model details'
      }
    } finally {
      if (!activeController.signal.aborted) {
        loading.value = false
      }
      if (controller === activeController) {
        controller = null
      }
    }
  }

  watch(
    () => [toValue(options.open), toValue(options.modelId)] as const,
    () => {
      void refresh()
    },
    { immediate: true }
  )

  onUnmounted(() => {
    controller?.abort()
  })

  return { data, loading, error, refresh }
}
