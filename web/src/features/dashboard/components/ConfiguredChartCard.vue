<script setup lang="ts">
import {
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip
} from 'chart.js'
import { Bar, Line } from 'vue-chartjs'
import { chartData, chartOptions, type DashboardChartTheme } from '@/features/dashboard/utils/chartHelpers'
import type { ConfiguredChart } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Tooltip, Legend)

defineProps<{
  chart: ConfiguredChart
  theme: DashboardChartTheme
}>()
</script>

<template>
  <VCard class="chart-card">
    <div class="chart-card__header">
      <div>
        <p class="eyebrow">{{ chart.metric }}</p>
        <h2>{{ chart.title }}</h2>
      </div>
      <VChip size="small" variant="tonal">{{ chart.type }}</VChip>
    </div>

    <VAlert v-if="chart.error" density="compact" type="warning" variant="tonal">
      {{ chart.error }}
    </VAlert>

    <div v-else-if="chart.datasets.length === 0" class="empty-chart">
      No samples in this window
    </div>

    <div v-else class="chart-frame">
      <Line
        v-if="chart.type === 'line'"
        :data="chartData(chart)"
        :options="chartOptions(chart, theme)"
      />
      <Bar
        v-else
        :data="chartData(chart)"
        :options="chartOptions(chart, theme)"
      />
    </div>
  </VCard>
</template>
