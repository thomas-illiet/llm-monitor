<script setup lang="ts">
import { computed } from 'vue'
import ConfiguredChartCard from '@/features/dashboard/components/ConfiguredChartCard.vue'
import ModelCapabilityDoughnutCard from '@/features/dashboard/components/ModelCapabilityDoughnutCard.vue'
import ModelEventsTimelineCard from '@/features/dashboard/components/ModelEventsTimelineCard.vue'
import ModelStatusKpiCard from '@/features/dashboard/components/ModelStatusKpiCard.vue'
import { chartGridEntries, chartTheme } from '@/features/dashboard/utils/chartHelpers'
import type { ConfiguredChart, ModelEvent, ModelState } from '@/types'

const props = defineProps<{
  charts: ConfiguredChart[]
  events: ModelEvent[]
  isDark: boolean
  modelStatusHistory: ConfiguredChart
  models: ModelState[]
}>()

const emit = defineEmits<{
  openEvents: [modelId: string]
}>()

const theme = computed(() => chartTheme(props.isDark))
const chartEntries = computed(() => chartGridEntries(props.charts))
</script>

<template>
  <section class="charts-grid">
    <template v-for="entry in chartEntries" :key="entry.key">
      <ConfiguredChartCard v-if="entry.kind === 'chart'" :chart="entry.chart" :theme="theme" />
      <ModelStatusKpiCard
        v-else-if="entry.kind === 'model-status'"
        :model-status-history="modelStatusHistory"
        :theme="theme"
      />
      <ModelCapabilityDoughnutCard
        v-else-if="entry.kind === 'model-capability'"
        :models="models"
        :theme="theme"
      />
      <ModelEventsTimelineCard
        v-else
        :events="events"
        @open-events="emit('openEvents', $event)"
      />
    </template>
  </section>
</template>
