<script setup lang="ts">
import { computed } from 'vue'
import type { ModelEvent } from '@/types'

const page = defineModel<number>('page', { default: 1 })
const itemsPerPage = defineModel<number>('itemsPerPage', { default: 25 })
const props = withDefaults(defineProps<{
  events: ModelEvent[]
  total: number
  showModel?: boolean
  loading?: boolean
  height?: string | number
}>(), {
  showModel: true,
  loading: false
})

type EventTableHeader = {
  title: string
  key: string
  sortable: boolean
  align?: 'start' | 'end'
}

const headers = computed<EventTableHeader[]>(() => {
  const columns: EventTableHeader[] = [
    { title: 'Time', key: 'observed_at', sortable: false },
    { title: 'Status', key: 'status', sortable: false },
    { title: 'Source', key: 'source', sortable: false },
    { title: 'Event', key: 'event_type', sortable: false },
    { title: 'Details', key: 'details', sortable: false }
  ]
  if (props.showModel) {
    columns.splice(4, 0, { title: 'Model', key: 'model_id', sortable: false })
  }
  return columns
})

/** Formats event timestamps for dense timeline rows. */
function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium'
  }).format(new Date(value))
}

/** Maps event severity/status to Vuetify table chip colors. */
function eventColor(event: ModelEvent) {
  if (event.severity === 'error' || event.status === 'error') return 'error'
  if (event.severity === 'warning' || event.status === 'skipped' || event.status === 'missing') return 'warning'
  return 'success'
}

/** Builds a compact scalar-only summary from event details. */
function detailSummary(event: ModelEvent) {
  if (!event.details) return ''
  const pairs = Object.entries(event.details)
    .filter(([, value]) => value !== '' && value !== null && value !== undefined && typeof value !== 'object')
    .slice(0, 6)
    .map(([key, value]) => `${key}: ${String(value)}`)
  return pairs.join(' · ')
}
</script>

<template>
  <VDataTableServer
    v-model:page="page"
    v-model:items-per-page="itemsPerPage"
    class="dashboard-data-table dashboard-table dashboard-table--events"
    density="compact"
    fixed-header
    hover
    item-value="id"
    :headers="headers"
    :height="height"
    :items="events"
    :items-length="total"
    :items-per-page-options="[10, 25, 50, 100]"
    :loading="loading"
  >
    <template #item.observed_at="{ item }">
      {{ formatTime(item.observed_at) }}
    </template>
    <template #item.status="{ item }">
      <VChip size="x-small" :color="eventColor(item)" variant="tonal">
        {{ item.status || item.severity }}
      </VChip>
    </template>
    <template #item.event_type="{ item }">
      <VChip size="x-small" :color="eventColor(item)" variant="tonal">
        {{ item.event_type }}
      </VChip>
    </template>
    <template #item.model_id="{ item }">
      <span class="model-name">{{ item.model_id }}</span>
    </template>
    <template #item.details="{ item }">
      <div class="event-detail-cell">
        <strong>{{ item.title }}</strong>
        <span>{{ item.message }}</span>
        <small v-if="detailSummary(item)">{{ detailSummary(item) }}</small>
      </div>
    </template>
    <template #no-data>
      <span class="empty-inline">No model event matches these filters</span>
    </template>
  </VDataTableServer>
</template>
