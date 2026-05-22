<script setup lang="ts">
import { computed, onMounted, onUnmounted, shallowRef } from 'vue'
import {
  compareNullableNumbers,
  formatCountdown,
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
  checkingModelIds?: string[]
}>()

const emit = defineEmits<{
  openCompare: [modelId: string]
  openDashboard: [modelId: string]
  openEvents: [modelId: string]
  openTechnicalDetails: [modelId: string]
  runCheck: [modelId: string]
}>()

const search = shallowRef('')
const sortBy = shallowRef([{ key: 'status_label', order: 'asc' as const }])
const now = shallowRef(Date.now())
let countdownTimer: number | undefined

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
  {
    title: 'Next check',
    key: 'next_check_at',
    sortable: true,
    sortRaw: (a: ModelInventoryRow, b: ModelInventoryRow) => compareNullableNumbers(a.next_check_timestamp, b.next_check_timestamp)
  },
  { title: 'Last seen', key: 'last_seen_at', sortable: true },
  { title: 'Action', key: 'actions', sortable: false, align: 'end' as const },
]

const lastRunByModel = computed(() => {
  return buildLastRunByModel(props.runs)
})

const tableRows = computed<ModelInventoryRow[]>(() => {
  return modelInventoryRows(props.models, lastRunByModel.value)
})

const checkingModels = computed(() => new Set(props.checkingModelIds ?? []))

onMounted(() => {
  countdownTimer = window.setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (countdownTimer !== undefined) window.clearInterval(countdownTimer)
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
      <template #item.next_check_at="{ item }">
        {{ formatCountdown(item.next_check_at, now) }}
      </template>
      <template #item.last_seen_at="{ item }">
        {{ formatTime(item.last_seen_at) }}
      </template>
      <template #item.actions="{ item }">
        <VMenu location="bottom end">
          <template #activator="{ props: menuProps }">
            <VBtn
              v-bind="menuProps"
              append-icon="mdi-chevron-down"
              class="model-action-button"
              size="small"
              variant="tonal"
              density="compact"
              :aria-label="`Actions for ${item.model_id}`"
              :loading="checkingModels.has(item.model_id)"
            >
              Action
            </VBtn>
          </template>
          <VList density="compact" min-width="160">
            <VListItem
              prepend-icon="mdi-play-circle-outline"
              :disabled="checkingModels.has(item.model_id)"
              @click="emit('runCheck', item.model_id)"
            >
              <VListItemTitle>Run check</VListItemTitle>
            </VListItem>
            <VListItem prepend-icon="mdi-view-dashboard-outline" @click="emit('openDashboard', item.model_id)">
              <VListItemTitle>Dashboard</VListItemTitle>
            </VListItem>
            <VListItem prepend-icon="mdi-compare-horizontal" @click="emit('openCompare', item.model_id)">
              <VListItemTitle>Compare</VListItemTitle>
            </VListItem>
            <VListItem prepend-icon="mdi-code-json" @click="emit('openTechnicalDetails', item.model_id)">
              <VListItemTitle>Technical details</VListItemTitle>
            </VListItem>
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
