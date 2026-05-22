<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import ConfiguredChartCard from '@/features/dashboard/components/ConfiguredChartCard.vue'
import { useModelDashboardData } from '@/features/dashboard/composables/useModelDashboardData'
import { chartTheme } from '@/features/dashboard/utils/chartHelpers'
import { comparisonCharts, comparisonMetrics } from '@/features/dashboard/utils/modelComparison'
import { formatTime, modelIdentity, statusColor } from '@/features/dashboard/utils/modelInventory'
import type { KpiRangeValue, ModelState } from '@/types'

const selectedReferenceModelId = defineModel<string | null>('modelId', { default: null })

const props = defineProps<{
  isDark: boolean
  kpiRange: KpiRangeValue
  models: ModelState[]
}>()

const selectedComparedModelId = shallowRef<string | null>(null)

const theme = computed(() => chartTheme(props.isDark))
const modelOptionsKey = computed(() => props.models.map(model => `${modelIdentity(model)}:${model.capability}`).join('\u0000'))
const referenceModels = computed(() => props.models.filter(model => model.capability !== 'skip'))
const selectedReferenceModel = computed(() => {
  return props.models.find(model => modelIdentity(model) === selectedReferenceModelId.value) ?? null
})
const compatibleComparedModels = computed(() => {
  const reference = selectedReferenceModel.value
  if (!reference) return []
  return props.models.filter(model => {
    return modelIdentity(model) !== modelIdentity(reference) && model.capability === reference.capability && model.capability !== 'skip'
  })
})
const selectedComparedModel = computed(() => {
  return props.models.find(model => modelIdentity(model) === selectedComparedModelId.value) ?? null
})
const {
  data: referenceData,
  loading: referenceLoading,
  error: referenceError,
  refresh: refreshReference
} = useModelDashboardData({
  providerId: () => selectedReferenceModel.value?.provider_id ?? null,
  modelId: () => selectedReferenceModel.value?.model_id ?? null,
  kpiRange: () => props.kpiRange
})

const {
  data: comparedData,
  loading: comparedLoading,
  error: comparedError,
  refresh: refreshCompared
} = useModelDashboardData({
  providerId: () => selectedComparedModel.value?.provider_id ?? null,
  modelId: () => selectedComparedModel.value?.model_id ?? null,
  kpiRange: () => props.kpiRange
})
const referenceModelOptions = computed(() => referenceModels.value.map(modelOption))
const comparedModelOptions = computed(() => compatibleComparedModels.value.map(modelOption))
const loading = computed(() => referenceLoading.value || comparedLoading.value)
const hasComparisonData = computed(() => {
  return referenceData.value && comparedData.value &&
    modelIdentity(referenceData.value.model) === selectedReferenceModelId.value &&
    modelIdentity(comparedData.value.model) === selectedComparedModelId.value
})
const summaryModels = computed(() => {
  return [
    { label: 'Reference', model: referenceData.value?.model ?? selectedReferenceModel.value },
    { label: 'Compared', model: comparedData.value?.model ?? selectedComparedModel.value }
  ].filter((entry): entry is { label: string, model: ModelState } => entry.model !== null)
})
const metrics = computed(() => {
  if (!referenceData.value || !comparedData.value || !hasComparisonData.value) return []
  return comparisonMetrics(referenceData.value.kpis, comparedData.value.kpis, referenceData.value.model.capability)
})
const charts = computed(() => {
  if (!referenceData.value || !comparedData.value || !hasComparisonData.value) return []
  return comparisonCharts(
    referenceData.value.charts,
    comparedData.value.charts,
    `${referenceData.value.model.provider_id}/${referenceData.value.model.model_id}`,
    `${comparedData.value.model.provider_id}/${comparedData.value.model.model_id}`
  )
})

watch([modelOptionsKey, selectedReferenceModelId], () => {
  if (referenceModels.value.length === 0) {
    selectedReferenceModelId.value = null
    selectedComparedModelId.value = null
    return
  }

  if (!selectedReferenceModelId.value || !referenceModels.value.some(model => modelIdentity(model) === selectedReferenceModelId.value)) {
    selectedReferenceModelId.value = preferredReferenceModelId()
  }

  if (!selectedComparedModelId.value || !compatibleComparedModels.value.some(model => modelIdentity(model) === selectedComparedModelId.value)) {
    selectedComparedModelId.value = preferredComparedModelId()
  }
}, { immediate: true })

/** Refreshes both selected model dashboard payloads together. */
function refresh() {
  void refreshReference()
  void refreshCompared()
}

/** Picks the initial reference model, favoring runnable active models. */
function preferredReferenceModelId() {
  const preferred = referenceModels.value.find(model => model.status === 'active' && !model.excluded) ?? referenceModels.value[0]
  return preferred ? modelIdentity(preferred) : null
}

/** Picks the initial compared model from the reference capability. */
function preferredComparedModelId() {
  const preferred = compatibleComparedModels.value.find(model => model.status === 'active' && !model.excluded) ?? compatibleComparedModels.value[0]
  return preferred ? modelIdentity(preferred) : null
}

function modelOption(model: ModelState) {
  return {
    subtitle: model.capability,
    title: `${model.provider_id}/${model.model_id}`,
    value: modelIdentity(model)
  }
}
</script>

<template>
  <section class="model-comparison-panel">
    <div class="model-comparison-panel__header">
      <div>
        <p class="eyebrow">Model A/B</p>
        <h2>{{ selectedReferenceModel ? `${selectedReferenceModel.provider_id}/${selectedReferenceModel.model_id}` : 'No model selected' }}</h2>
      </div>
      <div class="model-comparison-panel__actions">
        <VSelect
          v-model="selectedReferenceModelId"
          aria-label="Select reference model"
          class="model-comparison-panel__select"
          density="compact"
          hide-details
          item-title="title"
          item-value="value"
          :disabled="referenceModels.length === 0"
          :items="referenceModelOptions"
          variant="outlined"
        />
        <VSelect
          v-model="selectedComparedModelId"
          aria-label="Select compared model"
          class="model-comparison-panel__select"
          density="compact"
          hide-details
          item-title="title"
          item-value="value"
          :disabled="compatibleComparedModels.length === 0"
          :items="comparedModelOptions"
          variant="outlined"
        />
        <VBtn
          icon="mdi-refresh"
          size="small"
          variant="tonal"
          :disabled="!selectedReferenceModelId || !selectedComparedModelId || loading"
          :loading="loading"
          aria-label="Refresh model comparison"
          @click="refresh"
        />
      </div>
    </div>

    <div v-if="referenceModels.length === 0" class="model-comparison-panel__empty">
      No comparable model snapshot yet
    </div>

    <div v-else-if="selectedReferenceModel && compatibleComparedModels.length === 0" class="model-comparison-panel__empty">
      No other {{ selectedReferenceModel.capability }} model is available for comparison
    </div>

    <template v-else>
      <div v-if="referenceError || comparedError" class="model-comparison-panel__alerts">
        <VAlert v-if="referenceError" density="comfortable" type="error" variant="tonal">
          Reference model: {{ referenceError }}
        </VAlert>
        <VAlert v-if="comparedError" density="comfortable" type="error" variant="tonal">
          Compared model: {{ comparedError }}
        </VAlert>
      </div>

      <div v-if="summaryModels.length > 0" class="model-comparison-panel__summary-grid">
        <div v-for="entry in summaryModels" :key="entry.label" class="model-comparison-panel__summary">
          <p class="eyebrow">{{ entry.label }}</p>
          <strong>{{ entry.model.provider_id }}/{{ entry.model.model_id }}</strong>
          <div class="model-comparison-panel__summary-chips">
            <VChip size="x-small" :color="statusColor(entry.model)" variant="tonal">
              {{ entry.model.excluded ? 'excluded' : entry.model.status }}
            </VChip>
            <VChip size="x-small" variant="tonal">{{ entry.model.capability }}</VChip>
          </div>
          <span>First seen {{ formatTime(entry.model.first_seen_at) }}</span>
          <span>Last seen {{ formatTime(entry.model.last_seen_at) }}</span>
        </div>
      </div>

      <div v-if="loading && !hasComparisonData" class="loading-state model-comparison-panel__loading">
        <div class="loading-dots">
          <span /><span /><span />
        </div>
        <span>Loading comparison telemetry</span>
      </div>

      <template v-else-if="hasComparisonData && referenceData && comparedData">
        <section class="model-comparison-panel__metric-grid">
          <div
            v-for="metric in metrics"
            :key="metric.key"
            class="model-comparison-panel__metric"
          >
            <div class="model-comparison-panel__metric-header">
              <p>{{ metric.label }}</p>
              <VChip
                size="x-small"
                variant="tonal"
                :color="metric.delta.tone === 'positive' ? 'success' : metric.delta.tone === 'negative' ? 'error' : undefined"
              >
                {{ metric.delta.label }}
              </VChip>
            </div>
            <div class="model-comparison-panel__metric-values">
              <span>A <strong>{{ metric.referenceLabel }}</strong></span>
              <span>B <strong>{{ metric.comparedLabel }}</strong></span>
            </div>
            <small :class="`model-comparison-panel__delta model-comparison-panel__delta--${metric.delta.tone}`">
              B - A: {{ metric.delta.label }} / {{ metric.delta.percentageLabel }}
            </small>
          </div>
        </section>

        <section class="charts-grid model-comparison-panel__charts">
          <ConfiguredChartCard
            v-for="chart in charts"
            :key="chart.id"
            :chart="chart"
            :theme="theme"
          />
        </section>
      </template>

      <div v-else class="model-comparison-panel__empty">
        No comparison data yet
      </div>
    </template>
  </section>
</template>
