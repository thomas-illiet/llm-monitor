<script setup lang="ts">
import { computed } from 'vue'
import type { KpiRangePreset, KpiRangeValue } from '@/types'

const selectedRange = defineModel<KpiRangeValue>({ required: true })

const props = defineProps<{
  presets: readonly KpiRangePreset[]
  loading?: boolean
}>()

const activePreset = computed(() => {
  return props.presets.find(preset => preset.value === selectedRange.value)
})
</script>

<template>
  <section class="kpi-range-bar" aria-label="KPI time window">
    <div class="kpi-range-bar__label">
      <p class="eyebrow">KPI window</p>
      <strong>{{ activePreset?.label }}</strong>
    </div>

    <VSelect
      v-model="selectedRange"
      aria-label="Select KPI window"
      class="kpi-range-select"
      density="compact"
      hide-details
      item-title="label"
      item-value="value"
      :disabled="loading"
      :items="presets"
      single-line
      variant="outlined"
    />
  </section>
</template>
