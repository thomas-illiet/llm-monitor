<script setup lang="ts">
import { computed } from 'vue'
import ConfiguredChartCard from '@/features/dashboard/components/ConfiguredChartCard.vue'
import ModelStatusKpiCard from '@/features/dashboard/components/ModelStatusKpiCard.vue'
import { chartGridEntries, chartTheme } from '@/features/dashboard/utils/chartHelpers'
import type { ConfiguredChart } from '@/types'

const props = defineProps<{
  charts: ConfiguredChart[]
  isDark: boolean
  modelStatusHistory: ConfiguredChart
}>()

const theme = computed(() => chartTheme(props.isDark))
const chartEntries = computed(() => chartGridEntries(props.charts))
</script>

<template>
  <section class="charts-grid">
    <template v-for="entry in chartEntries" :key="entry.key">
      <ConfiguredChartCard v-if="entry.kind === 'chart'" :chart="entry.chart" :theme="theme" />
      <ModelStatusKpiCard
        v-else
        :model-status-history="modelStatusHistory"
        :theme="theme"
      />
    </template>
  </section>
</template>
