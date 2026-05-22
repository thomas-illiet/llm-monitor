<script setup lang="ts">
import ModelEventList from '@/features/dashboard/components/ModelEventList.vue'
import { useModelEvents } from '@/features/dashboard/composables/useModelEvents'

const open = defineModel<boolean>({ default: false })
const props = defineProps<{
  modelId: string | null
}>()

const {
  events,
  total,
  page,
  itemsPerPage,
  loading,
  error,
  selectedStatuses,
  selectedSources,
  selectedEventTypes,
  filterOptions,
  selectedFilterCount,
  eventCountLabel,
  clearFilters
} = useModelEvents({
  open,
  modelIdentity: () => props.modelId
})
</script>

<template>
  <VDialog v-model="open" width="96vw" max-width="1440">
    <VCard>
      <VCardTitle class="pt-4 px-4">
        <p class="eyebrow">Events</p>
        <h2 class="model-name">{{ modelId }}</h2>
      </VCardTitle>
      <VDivider />
      <VCardText class="pa-0">
        <VAlert v-if="error" class="ma-4" density="comfortable" type="error" variant="tonal">
          {{ error }}
        </VAlert>
        <div v-else class="events-dialog__body">
          <div class="events-dialog__filters">
            <VSelect
              v-model="selectedStatuses"
              class="events-dialog__filter"
              density="compact"
              variant="outlined"
              label="Status"
              :items="filterOptions.statuses"
              multiple
              chips
              closable-chips
              clearable
              hide-details
            />
            <VSelect
              v-model="selectedSources"
              class="events-dialog__filter"
              density="compact"
              variant="outlined"
              label="Source"
              :items="filterOptions.sources"
              multiple
              chips
              closable-chips
              clearable
              hide-details
            />
            <VSelect
              v-model="selectedEventTypes"
              class="events-dialog__filter"
              density="compact"
              variant="outlined"
              label="Event"
              :items="filterOptions.event_types"
              multiple
              chips
              closable-chips
              clearable
              hide-details
            />
            <VBtn
              size="small"
              variant="text"
              prepend-icon="mdi-filter-remove-outline"
              :disabled="selectedFilterCount === 0"
              @click="clearFilters"
            >
              Clear
            </VBtn>
            <VChip size="small" variant="tonal">{{ eventCountLabel }}</VChip>
          </div>
          <ModelEventList
            v-model:page="page"
            v-model:items-per-page="itemsPerPage"
            :events="events"
            :loading="loading"
            :show-model="false"
            :total="total"
            height="480"
          />
        </div>
      </VCardText>
      <VCardActions class="justify-end pa-3">
        <VBtn variant="text" @click="open = false">Close</VBtn>
      </VCardActions>
    </VCard>
  </VDialog>
</template>
