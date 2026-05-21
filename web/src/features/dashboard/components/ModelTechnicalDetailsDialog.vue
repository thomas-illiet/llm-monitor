<script setup lang="ts">
import { computed } from 'vue'
import { useModelTechnicalDetails } from '@/features/dashboard/composables/useModelTechnicalDetails'
import { formatTime, statusColor } from '@/features/dashboard/utils/modelInventory'
import type { ModelState } from '@/types'

const open = defineModel<boolean>({ default: false })
const props = defineProps<{
  modelId: string | null
}>()

interface MetadataRow {
  key: string
  value: string
}

interface SummaryRow {
  label: string
  value: string
}

const { data, loading, error, refresh } = useModelTechnicalDetails({
  open,
  modelId: () => props.modelId
})

const model = computed(() => data.value?.model ?? null)
const providerMetadata = computed(() => data.value?.provider_metadata ?? {})
const providerMetadataRows = computed<MetadataRow[]>(() => {
  return Object.entries(providerMetadata.value)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => ({
      key,
      value: formatMetadataValue(value)
    }))
})
const summaryRows = computed<SummaryRow[]>(() => {
  const selected = model.value
  if (!selected) return []
  return [
    { label: 'First seen', value: formatTime(selected.first_seen_at) },
    { label: 'Last seen', value: formatTime(selected.last_seen_at) },
    { label: 'Next check', value: formatTime(selected.next_check_at) },
    { label: 'Missing since', value: formatTime(selected.missing_since) },
    { label: 'Last probe', value: formatTime(selected.last_probe_at) },
    { label: 'Excluded', value: selected.excluded ? 'yes' : 'no' },
    { label: 'Skip reason', value: selected.skip_reason || 'n/a' }
  ]
})

function refreshDetails() {
  void refresh()
}

function formatMetadataValue(value: unknown) {
  if (value === null) return 'null'
  if (value === undefined) return 'n/a'
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  return String(value)
}

function modelStatusLabel(selected: ModelState) {
  return selected.excluded ? 'excluded' : selected.status
}
</script>

<template>
  <VDialog v-model="open" width="96vw" max-width="1040" scrollable>
    <VCard class="model-dashboard-dialog technical-details-dialog">
      <VCardTitle class="model-dashboard-dialog__title">
        <div>
          <p class="eyebrow">Technical details</p>
          <h2 class="model-name">{{ model?.model_id ?? modelId ?? 'Model details' }}</h2>
        </div>
        <div class="technical-details-dialog__title-actions">
          <VBtn
            icon="mdi-refresh"
            size="small"
            variant="tonal"
            :disabled="loading || !modelId"
            :loading="loading"
            aria-label="Refresh model technical details"
            @click="refreshDetails"
          />
          <VBtn
            icon="mdi-close"
            size="small"
            variant="text"
            aria-label="Close technical details"
            @click="open = false"
          />
        </div>
      </VCardTitle>
      <VDivider />
      <VCardText class="model-dashboard-dialog__body technical-details-dialog__body">
        <VAlert v-if="error" density="comfortable" type="error" variant="tonal">
          {{ error }}
        </VAlert>

        <div v-else-if="loading && !data" class="dialog-loading">
          <VProgressCircular indeterminate size="22" />
          <span>Loading provider metadata</span>
        </div>

        <template v-else-if="data && model">
          <section class="technical-details-dialog__summary">
            <div class="technical-details-dialog__identity">
              <span class="model-name">{{ model.model_id }}</span>
              <div class="technical-details-dialog__chips">
                <VChip size="x-small" :color="statusColor(model)" variant="tonal">
                  {{ modelStatusLabel(model) }}
                </VChip>
                <VChip size="x-small" variant="tonal">{{ model.capability }}</VChip>
              </div>
            </div>
            <dl class="technical-details-dialog__summary-grid">
              <div
                v-for="row in summaryRows"
                :key="row.label"
                class="technical-details-dialog__summary-item"
              >
                <dt>{{ row.label }}</dt>
                <dd>{{ row.value }}</dd>
              </div>
            </dl>
          </section>

          <section class="technical-details-dialog__metadata">
            <div class="technical-details-dialog__section-header">
              <p class="eyebrow">Provider metadata</p>
              <VChip size="small" variant="tonal">{{ providerMetadataRows.length }}</VChip>
            </div>

            <VTable v-if="providerMetadataRows.length > 0" class="technical-details-dialog__table" density="compact">
              <thead>
                <tr>
                  <th>Field</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in providerMetadataRows" :key="row.key">
                  <td class="technical-details-dialog__key">{{ row.key }}</td>
                  <td><pre>{{ row.value }}</pre></td>
                </tr>
              </tbody>
            </VTable>
            <div v-else class="technical-details-dialog__empty">
              No provider metadata has been captured for this model yet
            </div>
          </section>
        </template>
      </VCardText>
    </VCard>
  </VDialog>
</template>
