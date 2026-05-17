<script setup lang="ts">
import type { RecentRun } from '@/types'

defineProps<{
  runs: RecentRun[]
}>()

const headers = [
  { title: 'Started', key: 'started_at', sortable: true },
  { title: 'Kind', key: 'kind', sortable: true },
  { title: 'Status', key: 'status', sortable: false },
  { title: 'Latency', key: 'latency_ms', sortable: true },
  { title: 'Tokens', key: 'tokens', sortable: false },
  { title: 'Error', key: 'error', sortable: false }
]

/** Formats run timestamps for dense model telemetry rows. */
function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium'
  }).format(new Date(value))
}

/** Builds the compact token summary for chat and embedding probes. */
function tokenSummary(run: RecentRun) {
  const parts = [
    run.input_tokens === undefined ? '' : `${run.input_tokens} in`,
    run.output_tokens === undefined ? '' : `${run.output_tokens} out`,
    run.total_tokens === undefined ? '' : `${run.total_tokens} total`
  ].filter(Boolean)
  return parts.length > 0 ? parts.join(' / ') : 'n/a'
}
</script>

<template>
  <VCard class="table-card model-runs-card">
    <div class="table-card__header">
      <div>
        <p class="eyebrow">Probes</p>
        <h2>Recent model runs</h2>
      </div>
      <VChip size="small" variant="tonal">{{ runs.length }}</VChip>
    </div>

    <VDataTable
      class="dashboard-data-table dashboard-table dashboard-table--runs"
      density="compact"
      hover
      :headers="headers"
      :items="runs"
      :items-per-page="10"
      :items-per-page-options="[10, 25, 50, { value: -1, title: 'All' }]"
    >
      <template #item.started_at="{ item }">
        {{ formatTime(item.started_at) }}
      </template>
      <template #item.kind="{ item }">
        <VChip size="x-small" variant="tonal">{{ item.kind }}</VChip>
      </template>
      <template #item.status="{ item }">
        <VChip size="x-small" :color="item.ok ? 'success' : 'error'" variant="tonal">
          {{ item.ok ? 'ok' : `error ${item.status_code || '?'}` }}
        </VChip>
      </template>
      <template #item.latency_ms="{ item }">
        {{ Math.round(item.latency_ms) }} ms
      </template>
      <template #item.tokens="{ item }">
        {{ tokenSummary(item) }}
      </template>
      <template #item.error="{ item }">
        <span class="error-cell">{{ item.error || 'n/a' }}</span>
      </template>
      <template #no-data>
        <span class="empty-inline">No run in this window</span>
      </template>
    </VDataTable>
  </VCard>
</template>
