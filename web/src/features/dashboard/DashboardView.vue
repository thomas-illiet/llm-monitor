<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import StatusHeader from '@/features/dashboard/components/StatusHeader.vue'
import PublicStatusPanel from '@/features/dashboard/components/PublicStatusPanel.vue'
import KpiRangeSelector from '@/features/dashboard/components/KpiRangeSelector.vue'
import KpiCards from '@/features/dashboard/components/KpiCards.vue'
import ConfiguredCharts from '@/features/dashboard/components/ConfiguredCharts.vue'
import ModelComparisonDialog from '@/features/dashboard/components/ModelComparisonDialog.vue'
import ModelDashboardDialog from '@/features/dashboard/components/ModelDashboardDialog.vue'
import ModelInventoryTable from '@/features/dashboard/components/ModelInventoryTable.vue'
import ModelTechnicalDetailsDialog from '@/features/dashboard/components/ModelTechnicalDetailsDialog.vue'
import ModelEventsDialog from '@/features/dashboard/components/ModelEventsDialog.vue'
import { useDashboardData } from '@/features/dashboard/composables/useDashboardData'
import { useManualChecks } from '@/features/dashboard/composables/useManualChecks'
import {
  KPI_RANGE_PRESETS,
  clampKpiRangeValue,
  filterKpiRangePresets,
  usePersistentKpiRange
} from '@/features/dashboard/composables/usePersistentKpiRange'
import { usePersistentTheme } from '@/features/dashboard/composables/usePersistentTheme'

const { selectedKpiRange } = usePersistentKpiRange()
const { data, loading, error, refresh } = useDashboardData({ kpiRange: selectedKpiRange })
const {
  globalChecking,
  checkingModelIds,
  error: manualCheckError,
  runAllChecks,
  runModelCheck
} = useManualChecks({ onComplete: refresh })
const hasInitialData = computed(() => data.value !== null)
const { isDark, toggleTheme } = usePersistentTheme()
const selectedDashboardModel = shallowRef<string | null>(null)
const dashboardDialogOpen = shallowRef(false)
const selectedComparisonModel = shallowRef<string | null>(null)
const comparisonDialogOpen = shallowRef(false)
const selectedTechnicalModel = shallowRef<string | null>(null)
const technicalDetailsDialogOpen = shallowRef(false)
const selectedEventsModel = shallowRef<string | null>(null)
const eventsDialogOpen = shallowRef(false)

const publicStatus = computed(() => {
  const status = data.value?.status
  if (!status) return undefined
  return {
    ...status,
    ok: status.http_ok && status.inactive_models === 0
  }
})

const siteName = computed(() => data.value?.config.site_name || 'LLM Service Monitor')

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

watch(siteName, (name) => {
  document.title = name
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

/** Opens the model comparison dialog with the selected inventory row as model A. */
function openModelComparison(modelId: string) {
  selectedComparisonModel.value = modelId
  comparisonDialogOpen.value = true
}

/** Opens the provider metadata dialog for the selected inventory row. */
function openModelTechnicalDetails(modelId: string) {
  selectedTechnicalModel.value = modelId
  technicalDetailsDialogOpen.value = true
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
          :site-name="siteName"
          :loading="loading"
          :checking="globalChecking"
          :is-dark="isDark"
          @refresh="refresh"
          @run-checks="runAllChecks"
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
        <VAlert
          v-if="manualCheckError"
          class="app-alert"
          density="comfortable"
          type="error"
          variant="tonal"
        >
          {{ manualCheckError }}
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
            :events="data.events"
            :is-dark="isDark"
            :model-status-history="data.model_status_history"
            :models="data.models"
            @open-events="openModelEvents"
          />

          <ModelInventoryTable
            :models="data.models"
            :runs="data.runs"
            :checking-model-ids="checkingModelIds"
            @open-compare="openModelComparison"
            @open-dashboard="openModelDashboard"
            @open-events="openModelEvents"
            @open-technical-details="openModelTechnicalDetails"
            @run-check="runModelCheck"
          />
          <ModelComparisonDialog
            v-model="comparisonDialogOpen"
            v-model:model-id="selectedComparisonModel"
            :is-dark="isDark"
            :kpi-range="selectedKpiRange"
            :models="data.models"
          />
          <ModelDashboardDialog
            v-model="dashboardDialogOpen"
            v-model:model-id="selectedDashboardModel"
            :is-dark="isDark"
            :kpi-range="selectedKpiRange"
            :models="data.models"
          />
          <ModelTechnicalDetailsDialog
            v-model="technicalDetailsDialogOpen"
            :model-id="selectedTechnicalModel"
          />
          <ModelEventsDialog v-model="eventsDialogOpen" :model-id="selectedEventsModel" />
        </template>
      </div>
    </VMain>
  </VApp>
</template>
