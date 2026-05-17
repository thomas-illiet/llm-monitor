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

    <VBtnToggle
      v-model="selectedRange"
      class="kpi-range-toggle"
      density="compact"
      mandatory
      selected-class="kpi-range-toggle__button--active"
    >
      <VBtn
        v-for="preset in presets"
        :key="preset.value"
        class="kpi-range-toggle__button"
        :disabled="loading"
        :value="preset.value"
        size="small"
        variant="text"
      >
        {{ preset.label }}
      </VBtn>
    </VBtnToggle>
  </section>
</template>
