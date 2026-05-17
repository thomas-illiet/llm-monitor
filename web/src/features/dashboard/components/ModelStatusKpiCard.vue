<script setup lang="ts">
import ConfiguredChartCard from '@/features/dashboard/components/ConfiguredChartCard.vue'
import type { DashboardChartTheme } from '@/features/dashboard/utils/chartHelpers'
import type { ConfiguredChart } from '@/types'

defineProps<{
  modelStatusHistory: ConfiguredChart
  theme: DashboardChartTheme
}>()
</script>

<template>
  <ConfiguredChartCard :chart="modelStatusHistory" :theme="theme" />
</template>
