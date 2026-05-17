<script setup lang="ts">
import { computed, watch } from 'vue'
import ConfiguredChartCard from '@/features/dashboard/components/ConfiguredChartCard.vue'
import KpiCards from '@/features/dashboard/components/KpiCards.vue'
import ModelRunsTable from '@/features/dashboard/components/ModelRunsTable.vue'
import { useModelDashboardData } from '@/features/dashboard/composables/useModelDashboardData'
import { chartTheme } from '@/features/dashboard/utils/chartHelpers'
import { formatTime, statusColor } from '@/features/dashboard/utils/modelInventory'
import type { KpiRangeValue, ModelState } from '@/types'

const selectedModelId = defineModel<string | null>('modelId', { default: null })

const props = defineProps<{
  isDark: boolean
  kpiRange: KpiRangeValue
  models: ModelState[]
}>()

const {
  data,
  loading,
  error,
  refresh
} = useModelDashboardData({
  modelId: selectedModelId,
  kpiRange: () => props.kpiRange
})

const theme = computed(() => chartTheme(props.isDark))
const modelIdsKey = computed(() => props.models.map(model => model.model_id).join('\u0000'))
const modelOptions = computed(() => props.models.map(model => ({
  title: model.model_id,
  value: model.model_id
})))

const selectedModel = computed(() => {
  return data.value?.model ?? props.models.find(model => model.model_id === selectedModelId.value) ?? null
})

watch(modelIdsKey, () => {
  const modelIds = new Set(props.models.map(model => model.model_id))
  if (props.models.length === 0) {
    selectedModelId.value = null
    return
  }
  if (selectedModelId.value && modelIds.has(selectedModelId.value)) return
  selectedModelId.value = preferredModelId()
}, { immediate: true })

/** Picks the default model for the detail dashboard. */
function preferredModelId() {
  return props.models.find(model => model.status === 'active' && !model.excluded && model.capability !== 'skip')?.model_id ?? props.models[0]?.model_id ?? null
}
</script>

<template>
  <section class="model-dashboard-panel">
    <div class="model-dashboard-panel__header">
      <div>
        <p class="eyebrow">Selected model</p>
        <h2>{{ selectedModel?.model_id ?? 'No model selected' }}</h2>
      </div>
      <div class="model-dashboard-panel__actions">
        <VSelect
          v-model="selectedModelId"
          aria-label="Select model dashboard"
          class="model-dashboard-panel__select"
          density="compact"
          hide-details
          item-title="title"
          item-value="value"
          :disabled="models.length === 0"
          :items="modelOptions"
          variant="outlined"
        />
        <VBtn
          icon="mdi-refresh"
          size="small"
          variant="tonal"
          :disabled="!selectedModelId || loading"
          :loading="loading"
          aria-label="Refresh model dashboard"
          @click="refresh"
        />
      </div>
    </div>

    <div v-if="models.length === 0" class="model-dashboard-panel__empty">
      No model snapshot yet
    </div>

    <VAlert v-else-if="error" class="app-alert" density="comfortable" type="error" variant="tonal">
      {{ error }}
    </VAlert>

    <div v-else-if="loading && !data" class="loading-state model-dashboard-panel__loading">
      <div class="loading-dots">
        <span /><span /><span />
      </div>
      <span>Loading model telemetry</span>
    </div>

    <template v-else-if="data">
      <div class="model-dashboard-panel__summary">
        <VChip size="small" :color="statusColor(data.model)" variant="tonal">
          {{ data.model.excluded ? 'excluded' : data.model.status }}
        </VChip>
        <VChip size="small" variant="tonal">{{ data.model.capability }}</VChip>
        <span>First seen {{ formatTime(data.model.first_seen_at) }}</span>
        <span>Last seen {{ formatTime(data.model.last_seen_at) }}</span>
      </div>

      <KpiCards :kpis="data.kpis" :slo="data.slo" />

      <section class="charts-grid model-dashboard-panel__charts">
        <ConfiguredChartCard
          v-for="chart in data.charts"
          :key="chart.id"
          :chart="chart"
          :theme="theme"
        />
      </section>

      <ModelRunsTable :runs="data.runs" />
    </template>
  </section>
</template>
