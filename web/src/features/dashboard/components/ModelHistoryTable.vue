<script setup lang="ts">
import type { ModelEvent, RecentAlert } from '@/types'

defineProps<{
  events: ModelEvent[]
  alerts: RecentAlert[]
}>()

/** Renders model event and alert timestamps in the user's locale. */
function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(value))
}

/** Maps lifecycle events to subtle dashboard status colors. */
function eventColor(eventType: string) {
  if (eventType === 'added' || eventType === 'returned') {
    return 'success'
  }
  if (eventType === 'removed') {
    return 'warning'
  }
  return 'secondary'
}
</script>

<template>
  <VCard class="table-card">
    <div class="table-card__header">
      <div>
        <p class="eyebrow">History</p>
        <h2>Model changes</h2>
      </div>
      <VChip size="small" variant="tonal">{{ events.length }}</VChip>
    </div>

    <VTable class="dashboard-table" density="compact" fixed-header height="230">
      <thead>
        <tr>
          <th>Event</th>
          <th>Model</th>
          <th>Observed</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="event in events" :key="event.id">
          <td>
            <VChip size="x-small" :color="eventColor(event.event_type)" variant="tonal">
              {{ event.event_type }}
            </VChip>
          </td>
          <td class="model-name">{{ event.model_id }}</td>
          <td>{{ formatTime(event.observed_at) }}</td>
        </tr>
        <tr v-if="events.length === 0">
          <td colspan="3" class="empty-row">No model event yet</td>
        </tr>
      </tbody>
    </VTable>

    <div class="alert-list">
      <p class="eyebrow">Email alerts</p>
      <div v-if="alerts.length === 0" class="empty-inline">No alert sent yet</div>
      <div v-for="alert in alerts" :key="`${alert.type}-${alert.model_id}-${alert.sent_at}`" class="alert-line">
        <span>{{ alert.subject }}</span>
        <strong>{{ alert.model_id }}</strong>
        <small>{{ formatTime(alert.sent_at) }}</small>
      </div>
    </div>
  </VCard>
</template>
