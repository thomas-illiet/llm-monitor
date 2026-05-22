<script setup lang="ts">
import { formatBytes, formatPreciseDateTime } from '@/features/dashboard/utils/formatters'
import type { EmbeddingRecentRun, RecentRun } from '@/types'

defineProps<{
  runs: RecentRun[]
}>()

const headers = [
  { title: 'Started', key: 'started_at', sortable: true },
  { title: 'Capability', key: 'capability', sortable: true },
  { title: 'Status', key: 'status', sortable: false },
  { title: 'Latency', key: 'latency_ms', sortable: true },
  { title: 'Workload', key: 'workload', sortable: false },
  { title: 'Metrics', key: 'metrics', sortable: false },
  { title: 'Error', key: 'error', sortable: false }
]

/** Narrows a recent run to embedding-specific metadata. */
function isEmbeddingRun(run: RecentRun): run is EmbeddingRecentRun {
  return run.capability === 'embedding'
}

/** Builds the workload title for chat prompts and embedding fixtures. */
function workloadTitle(run: RecentRun) {
  if (isEmbeddingRun(run)) return run.fixture_path ?? 'Fixture not recorded'
  return run.prompt_id ? `Prompt ${run.prompt_id}` : 'Chat prompt'
}

/** Builds the workload detail without pretending unsupported metrics exist. */
function workloadDetail(run: RecentRun) {
  if (isEmbeddingRun(run)) return formatBytes(run.fixture_bytes, 'Size not recorded')
  return run.prompt_id ? 'Configured chat test' : 'Prompt ID not recorded'
}

/** Builds the measured metric summary for each probe capability. */
function metricSummary(run: RecentRun) {
  const parts = [
    run.input_tokens === undefined ? '' : `${run.input_tokens} input`,
    run.capability === 'chat' && run.output_tokens !== undefined ? `${run.output_tokens} output` : '',
    run.total_tokens === undefined ? '' : `${run.total_tokens} total`,
    isEmbeddingRun(run) && run.vector_dimensions !== undefined ? `${run.vector_dimensions} dimensions` : ''
  ].filter(Boolean)
  return parts.length > 0 ? parts.join(' / ') : 'Metrics not recorded'
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
        {{ formatPreciseDateTime(item.started_at) }}
      </template>
      <template #item.capability="{ item }">
        <VChip size="x-small" variant="tonal">{{ item.capability }}</VChip>
      </template>
      <template #item.status="{ item }">
        <VChip size="x-small" :color="item.ok ? 'success' : 'error'" variant="tonal">
          {{ item.ok ? 'ok' : `error ${item.status_code || '?'}` }}
        </VChip>
      </template>
      <template #item.latency_ms="{ item }">
        {{ Math.round(item.latency_ms) }} ms
      </template>
      <template #item.workload="{ item }">
        <div class="run-detail-cell">
          <strong>{{ workloadTitle(item) }}</strong>
          <span>{{ workloadDetail(item) }}</span>
        </div>
      </template>
      <template #item.metrics="{ item }">
        <span class="run-metrics-cell">{{ metricSummary(item) }}</span>
      </template>
      <template #item.error="{ item }">
        <span class="error-cell">{{ item.error || 'No error' }}</span>
      </template>
      <template #no-data>
        <span class="empty-inline">No run in this window</span>
      </template>
    </VDataTable>
  </VCard>
</template>
