<script setup lang="ts">
import { computed } from 'vue'
import {
  ArcElement,
  Chart as ChartJS,
  Legend,
  Tooltip
} from 'chart.js'
import type { ChartData, ChartOptions, TooltipItem } from 'chart.js'
import { Doughnut } from 'vue-chartjs'
import type { DashboardChartTheme } from '@/features/dashboard/utils/chartHelpers'
import type { ModelState } from '@/types'

ChartJS.register(ArcElement, Tooltip, Legend)

const props = defineProps<{
  models: ModelState[]
  theme: DashboardChartTheme
}>()

const capabilityPalette = ['#10a37f', '#2563eb']

const capabilityCounts = computed(() => {
  let chat = 0
  let embedding = 0

  for (const model of props.models) {
    if (model.excluded) continue
    if (model.capability === 'chat') chat += 1
    if (model.capability === 'embedding') embedding += 1
  }

  return { chat, embedding }
})

const total = computed(() => capabilityCounts.value.chat + capabilityCounts.value.embedding)
const totalLabel = computed(() => total.value === 1 ? 'model' : 'models')

const legendItems = computed(() => [
  { label: 'Chat', value: capabilityCounts.value.chat, color: capabilityPalette[0] },
  { label: 'Embedding', value: capabilityCounts.value.embedding, color: capabilityPalette[1] }
])

const doughnutData = computed<ChartData<'doughnut', number[], string>>(() => ({
  labels: ['Chat', 'Embedding'],
  datasets: [{
    data: [capabilityCounts.value.chat, capabilityCounts.value.embedding],
    backgroundColor: capabilityPalette,
    borderColor: props.theme.grid,
    borderWidth: 2,
    hoverOffset: 4
  }]
}))

const doughnutOptions = computed<ChartOptions<'doughnut'>>(() => {
  const currentTotal = total.value

  return {
    responsive: true,
    maintainAspectRatio: false,
    cutout: '68%',
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: props.theme.tooltipBg,
        titleColor: props.theme.tooltipText,
        bodyColor: props.theme.tooltipText,
        callbacks: {
          label(context: TooltipItem<'doughnut'>) {
            const value = Number(context.raw ?? 0)
            const percentage = currentTotal > 0 ? Math.round((value / currentTotal) * 100) : 0
            return `${context.label}: ${value} (${percentage}%)`
          }
        }
      }
    }
  }
})
</script>

<template>
  <VCard class="chart-card model-capability-card">
    <div class="chart-card__header">
      <div>
        <p class="eyebrow">inventory</p>
        <h2>Model capabilities</h2>
      </div>
      <VChip size="small" variant="tonal">doughnut</VChip>
    </div>

    <div v-if="total === 0" class="empty-chart">
      No runnable models
    </div>

    <div v-else class="model-capability-card__body">
      <div class="model-capability-card__chart">
        <Doughnut :data="doughnutData" :options="doughnutOptions" />
        <div class="model-capability-card__center" aria-hidden="true">
          <strong>{{ total }}</strong>
          <span>{{ totalLabel }}</span>
        </div>
      </div>

      <div class="model-capability-card__legend" aria-label="Model capability counts">
        <div
          v-for="item in legendItems"
          :key="item.label"
          class="model-capability-card__legend-item"
        >
          <span class="model-capability-card__swatch" :style="{ backgroundColor: item.color }" />
          <span class="model-capability-card__label">{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
        </div>
      </div>
    </div>
  </VCard>
</template>

<style scoped>
.model-capability-card__body {
  display: grid;
  min-height: 260px;
  align-items: center;
  gap: 18px;
  grid-template-columns: minmax(0, 1fr) minmax(128px, 0.45fr);
}

.model-capability-card__chart {
  position: relative;
  min-width: 0;
  height: 230px;
}

.model-capability-card__center {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  pointer-events: none;
}

.model-capability-card__center strong {
  color: var(--text-primary);
  font-size: 1.75rem;
  line-height: 1;
}

.model-capability-card__center span,
.model-capability-card__label {
  color: var(--text-secondary);
  font-size: 0.8125rem;
}

.model-capability-card__legend {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.model-capability-card__legend-item {
  display: grid;
  align-items: center;
  gap: 8px;
  grid-template-columns: 10px minmax(0, 1fr) auto;
}

.model-capability-card__legend-item strong {
  color: var(--text-primary);
  font-size: 0.95rem;
}

.model-capability-card__swatch {
  width: 10px;
  height: 10px;
  border-radius: 999px;
}

@media (max-width: 560px) {
  .model-capability-card__body {
    grid-template-columns: 1fr;
  }

  .model-capability-card__chart {
    height: 220px;
  }
}
</style>
