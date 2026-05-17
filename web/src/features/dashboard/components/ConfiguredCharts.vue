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
import { computed } from 'vue'
import { Bar, Line } from 'vue-chartjs'
import ModelStatusKpiCard from '@/features/dashboard/components/ModelStatusKpiCard.vue'
import { chartData, chartGridEntries, chartOptions, chartTheme } from '@/features/dashboard/utils/chartHelpers'
import type { ConfiguredChart, StatusSummary } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Tooltip, Legend)

const props = defineProps<{
  charts: ConfiguredChart[]
  isDark: boolean
  status: StatusSummary
  modelStatusHistory: ConfiguredChart
}>()

const theme = computed(() => chartTheme(props.isDark))
const chartEntries = computed(() => chartGridEntries(props.charts))
</script>

<template>
  <section class="charts-grid">
    <template v-for="entry in chartEntries" :key="entry.key">
      <VCard v-if="entry.kind === 'chart'" class="chart-card">
        <div class="chart-card__header">
          <div>
            <p class="eyebrow">{{ entry.chart.metric }}</p>
            <h2>{{ entry.chart.title }}</h2>
          </div>
          <VChip size="small" variant="tonal">{{ entry.chart.type }}</VChip>
        </div>

        <VAlert v-if="entry.chart.error" density="compact" type="warning" variant="tonal">
          {{ entry.chart.error }}
        </VAlert>

        <div v-else-if="entry.chart.datasets.length === 0" class="empty-chart">
          No samples in this window
        </div>

        <div v-else class="chart-frame">
          <Line
            v-if="entry.chart.type === 'line'"
            :data="chartData(entry.chart)"
            :options="chartOptions(entry.chart, theme)"
          />
          <Bar
            v-else
            :data="chartData(entry.chart)"
            :options="chartOptions(entry.chart, theme)"
          />
        </div>
      </VCard>
      <ModelStatusKpiCard
        v-else
        :status="status"
        :model-status-history="modelStatusHistory"
      />
    </template>
  </section>
</template>
