<script setup lang="ts">
import ModelComparisonPanel from '@/features/dashboard/components/ModelComparisonPanel.vue'
import type { KpiRangeValue, ModelState } from '@/types'

const open = defineModel<boolean>({ default: false })
const selectedModelId = defineModel<string | null>('modelId', { default: null })

defineProps<{
  isDark: boolean
  kpiRange: KpiRangeValue
  models: ModelState[]
}>()
</script>

<template>
  <VDialog v-model="open" width="96vw" max-width="1680" scrollable>
    <VCard class="model-dashboard-dialog">
      <VCardTitle class="model-dashboard-dialog__title">
        <div>
          <p class="eyebrow">Comparison</p>
          <h2>Model dashboards</h2>
        </div>
        <VBtn
          icon="mdi-close"
          size="small"
          variant="text"
          aria-label="Close model comparison"
          @click="open = false"
        />
      </VCardTitle>
      <VDivider />
      <VCardText class="model-dashboard-dialog__body">
        <ModelComparisonPanel
          v-if="open"
          v-model:model-id="selectedModelId"
          :is-dark="isDark"
          :kpi-range="kpiRange"
          :models="models"
        />
      </VCardText>
    </VCard>
  </VDialog>
</template>
