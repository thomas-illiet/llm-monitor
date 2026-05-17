<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import {
  compareNullableNumbers,
  formatTime,
  lastRunByModel as buildLastRunByModel,
  modelInventoryRows,
  statusColor
} from '@/features/dashboard/utils/modelInventory'
import type { ModelState, RecentRun } from '@/types'
import type { ModelInventoryRow } from '@/features/dashboard/utils/modelInventory'

const props = defineProps<{
  models: ModelState[]
  runs?: RecentRun[]
}>()

const emit = defineEmits<{
  openEvents: [modelId: string]
}>()

const search = shallowRef('')
const sortBy = shallowRef([{ key: 'status_label', order: 'asc' as const }])

const headers = [
  { title: 'Model', key: 'model_id', sortable: true },
  { title: 'Capability', key: 'capability', sortable: true },
  { title: 'Status', key: 'status_label', sortable: true },
  {
    title: 'Last check',
    key: 'last_check_label',
    sortable: true,
    sortRaw: (a: ModelInventoryRow, b: ModelInventoryRow) => compareNullableNumbers(a.last_check_at, b.last_check_at)
  },
  { title: 'First seen', key: 'first_seen_at', sortable: true },
  { title: 'Last seen', key: 'last_seen_at', sortable: true },
  { title: '', key: 'actions', sortable: false, align: 'end' as const },
]

const lastRunByModel = computed(() => {
  return buildLastRunByModel(props.runs)
})

const tableRows = computed<ModelInventoryRow[]>(() => {
  return modelInventoryRows(props.models, lastRunByModel.value)
})
</script>

<template>
  <VCard class="table-card">
    <div class="table-card__header">
      <div>
        <p class="eyebrow">Inventory</p>
        <h2>Available models</h2>
      </div>
      <div class="table-card__actions">
        <VTextField
          v-model="search"
          class="table-search"
          density="compact"
          variant="outlined"
          placeholder="Filter models…"
          hide-details
          clearable
        />
        <VChip size="small" variant="tonal">{{ models.length }}</VChip>
      </div>
    </div>

    <VDataTable
      class="dashboard-data-table"
      :headers="headers"
      :items="tableRows"
      :search="search"
      :items-per-page="10"
      :items-per-page-options="[10, 25, 50, { value: -1, title: 'All' }]"
      v-model:sort-by="sortBy"
      density="compact"
      hover
    >
      <template #item.model_id="{ item }">
        <span class="model-name">{{ item.model_id }}</span>
      </template>
      <template #item.capability="{ item }">
        <VChip size="x-small" variant="tonal">{{ item.capability }}</VChip>
      </template>
      <template #item.status_label="{ item }">
        <VChip size="x-small" :color="statusColor(item)" variant="tonal">
          {{ item.status_label }}
        </VChip>
      </template>
      <template #item.last_check_label="{ item }">
        <VChip
          size="x-small"
          :color="item.last_check_color"
          variant="tonal"
        >
          {{ item.last_check_label }}
        </VChip>
      </template>
      <template #item.first_seen_at="{ item }">
        {{ formatTime(item.first_seen_at) }}
      </template>
      <template #item.last_seen_at="{ item }">
        {{ formatTime(item.last_seen_at) }}
      </template>
      <template #item.actions="{ item }">
        <VMenu location="bottom end">
          <template #activator="{ props: menuProps }">
            <VBtn
              v-bind="menuProps"
              icon="mdi-dots-vertical"
              size="x-small"
              variant="text"
              density="compact"
              :aria-label="`Actions for ${item.model_id}`"
            />
          </template>
          <VList density="compact" min-width="160">
            <VListItem prepend-icon="mdi-timeline-clock-outline" @click="emit('openEvents', item.model_id)">
              <VListItemTitle>Events</VListItemTitle>
            </VListItem>
          </VList>
        </VMenu>
      </template>
      <template #no-data>
        <span class="empty-inline">No model snapshot yet</span>
      </template>
    </VDataTable>
  </VCard>
</template>
