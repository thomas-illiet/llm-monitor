<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import StatusHeader from '@/features/dashboard/components/StatusHeader.vue'
import PublicStatusPanel from '@/features/dashboard/components/PublicStatusPanel.vue'
import KpiRangeSelector from '@/features/dashboard/components/KpiRangeSelector.vue'
import KpiCards from '@/features/dashboard/components/KpiCards.vue'
import ConfiguredCharts from '@/features/dashboard/components/ConfiguredCharts.vue'
import ModelDashboardDialog from '@/features/dashboard/components/ModelDashboardDialog.vue'
import ModelInventoryTable from '@/features/dashboard/components/ModelInventoryTable.vue'
import ModelEventsDialog from '@/features/dashboard/components/ModelEventsDialog.vue'
import { useDashboardData } from '@/features/dashboard/composables/useDashboardData'
import {
  KPI_RANGE_PRESETS,
  clampKpiRangeValue,
  filterKpiRangePresets,
  usePersistentKpiRange
} from '@/features/dashboard/composables/usePersistentKpiRange'
import { usePersistentTheme } from '@/features/dashboard/composables/usePersistentTheme'

const { selectedKpiRange } = usePersistentKpiRange()
const { data, loading, error, refresh } = useDashboardData({ kpiRange: selectedKpiRange })
const hasInitialData = computed(() => data.value !== null)
const { isDark, toggleTheme } = usePersistentTheme()
const selectedDashboardModel = shallowRef<string | null>(null)
const dashboardDialogOpen = shallowRef(false)
const selectedEventsModel = shallowRef<string | null>(null)
const eventsDialogOpen = shallowRef(false)

const publicStatus = computed(() => {
  const status = data.value?.status
  if (!status) return undefined
  return {
    ...status,
    ok: status.http_ok && status.missing_models === 0
  }
})

const publicCharts = computed(() => {
  return data.value?.charts.filter(chart => {
    const id = chart.id.toLowerCase()
    const metric = chart.metric.toLowerCase()
    return !id.includes('auth') && !metric.startsWith('auth_')
  }) ?? []
})

const kpiRangePresets = computed(() => {
  return filterKpiRangePresets(KPI_RANGE_PRESETS, data.value?.config.retention.history_seconds ?? 0)
})

watch(kpiRangePresets, (presets) => {
  const clamped = clampKpiRangeValue(selectedKpiRange.value, presets)
  if (clamped !== selectedKpiRange.value) {
    selectedKpiRange.value = clamped
  }
}, { immediate: true })

/** Opens the model event dialog for the selected inventory row. */
function openModelEvents(modelId: string) {
  selectedEventsModel.value = modelId
  eventsDialogOpen.value = true
}

/** Opens the model dashboard dialog for the selected inventory row. */
function openModelDashboard(modelId: string) {
  selectedDashboardModel.value = modelId
  dashboardDialogOpen.value = true
}
</script>

<template>
  <VApp>
    <VMain class="app-main">
      <div class="app-shell">
        <div v-if="loading && hasInitialData" class="refresh-bar" />
        <StatusHeader
          :status="publicStatus"
          :generated-at="data?.generated_at"
          :loading="loading"
          :is-dark="isDark"
          @refresh="refresh"
          @toggle-theme="toggleTheme"
        />

        <VAlert
          v-if="error"
          class="app-alert"
          density="comfortable"
          type="error"
          variant="tonal"
        >
          {{ error }}
        </VAlert>

        <div v-if="loading && !hasInitialData" class="loading-state">
          <div class="loading-dots">
            <span /><span /><span />
          </div>
          <span>Loading service telemetry</span>
        </div>

        <template v-else-if="data">
          <PublicStatusPanel :status="publicStatus" :http="data.http" />
          <KpiRangeSelector
            v-model="selectedKpiRange"
            :loading="loading"
            :presets="kpiRangePresets"
          />
          <KpiCards :kpis="data.kpis" :slo="data.slo" />
          <ConfiguredCharts
            :charts="publicCharts"
            :is-dark="isDark"
            :model-status-history="data.model_status_history"
          />

          <ModelInventoryTable
            :models="data.models"
            :runs="data.runs"
            @open-dashboard="openModelDashboard"
            @open-events="openModelEvents"
          />
          <ModelDashboardDialog
            v-model="dashboardDialogOpen"
            v-model:model-id="selectedDashboardModel"
            :is-dark="isDark"
            :kpi-range="selectedKpiRange"
            :models="data.models"
          />
          <ModelEventsDialog v-model="eventsDialogOpen" :model-id="selectedEventsModel" />
        </template>
      </div>
    </VMain>
  </VApp>
</template>
