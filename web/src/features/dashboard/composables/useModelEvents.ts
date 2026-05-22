import { computed, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import type { ModelEvent, ModelEventFilterOptions, ModelEventsResponse } from '@/types'
import { parseModelIdentity } from '@/features/dashboard/utils/modelInventory'

interface UseModelEventsOptions {
  open: MaybeRefOrGetter<boolean>
  modelIdentity: MaybeRefOrGetter<string | null>
}

/** Creates the model event timeline state used by the events dialog. */
export function useModelEvents(options: UseModelEventsOptions) {
  const events = shallowRef<ModelEvent[]>([])
  const total = shallowRef(0)
  const page = shallowRef(1)
  const itemsPerPage = shallowRef(25)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const selectedStatuses = shallowRef<string[]>([])
  const selectedSources = shallowRef<string[]>([])
  const selectedEventTypes = shallowRef<string[]>([])
  const filterOptions = shallowRef<ModelEventFilterOptions>(emptyFilterOptions())

  const selectedFilterCount = computed(() => {
    return selectedStatuses.value.length + selectedSources.value.length + selectedEventTypes.value.length
  })

  const eventCountLabel = computed(() => total.value === 1 ? '1 event' : `${total.value} events`)

  watch(
    () => toValue(options.modelIdentity),
    () => {
      page.value = 1
      clearFilters()
      events.value = []
      total.value = 0
      filterOptions.value = emptyFilterOptions()
    }
  )

  watch(
    () => [
      selectedStatuses.value.join('\u0000'),
      selectedSources.value.join('\u0000'),
      selectedEventTypes.value.join('\u0000'),
      itemsPerPage.value
    ] as const,
    () => {
      page.value = 1
    }
  )

  watch(
    () => [
      toValue(options.open),
      toValue(options.modelIdentity),
      page.value,
      itemsPerPage.value,
      selectedStatuses.value.join('\u0000'),
      selectedSources.value.join('\u0000'),
      selectedEventTypes.value.join('\u0000')
    ] as const,
    async ([isOpen, identity], _previous, onCleanup) => {
      const parsed = parseModelIdentity(identity)
      if (!isOpen || !parsed) {
        loading.value = false
        return
      }
      const controller = new AbortController()
      onCleanup(() => controller.abort())
      loading.value = true
      error.value = null
      try {
        const limit = Math.max(1, itemsPerPage.value)
        const offset = (Math.max(1, page.value) - 1) * limit
        const params = new URLSearchParams({
          limit: String(limit),
          offset: String(offset)
        })
        appendFilterParams(params, 'status', selectedStatuses.value)
        appendFilterParams(params, 'source', selectedSources.value)
        appendFilterParams(params, 'event_type', selectedEventTypes.value)
        const response = await fetch(`/api/providers/${encodeURIComponent(parsed.providerId)}/models/${encodeURIComponent(parsed.modelKey)}/events?${params}`, {
          headers: { Accept: 'application/json' },
          signal: controller.signal
        })
        if (!response.ok) {
          throw new Error(`Model events API returned ${response.status}`)
        }
        const payload = await response.json() as ModelEventsResponse
        events.value = payload.events ?? []
        total.value = payload.total ?? 0
        filterOptions.value = payload.filters ?? emptyFilterOptions()
        if (total.value > 0 && events.value.length === 0 && offset >= total.value) {
          page.value = Math.max(1, Math.ceil(total.value / limit))
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          error.value = err instanceof Error ? err.message : 'Unable to load model events'
        }
      } finally {
        if (!controller.signal.aborted) {
          loading.value = false
        }
      }
    },
    { immediate: true }
  )

  /** Clears all selected timeline filters. */
  function clearFilters() {
    selectedStatuses.value = []
    selectedSources.value = []
    selectedEventTypes.value = []
  }

  return {
    events,
    total,
    page,
    itemsPerPage,
    loading,
    error,
    selectedStatuses,
    selectedSources,
    selectedEventTypes,
    filterOptions,
    selectedFilterCount,
    eventCountLabel,
    clearFilters
  }
}

/** Returns the empty filter option shape expected by the dialog. */
function emptyFilterOptions(): ModelEventFilterOptions {
  return {
    statuses: [],
    sources: [],
    event_types: []
  }
}

/** Appends repeated query parameters for multi-select filters. */
function appendFilterParams(params: URLSearchParams, key: string, values: string[]) {
  for (const value of values) {
    params.append(key, value)
  }
}
