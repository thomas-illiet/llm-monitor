<script setup lang="ts">
import { computed } from 'vue'
import { formatTimelineDateTime } from '@/features/dashboard/utils/formatters'
import { modelIdentity } from '@/features/dashboard/utils/modelInventory'
import type { ModelEvent } from '@/types'

const props = defineProps<{
  events: ModelEvent[]
}>()

const emit = defineEmits<{
  openEvents: [modelIdentity: string]
}>()

const visibleEvents = computed(() => props.events.slice(0, 15))

/** Maps event severity/status to Vuetify color tokens. */
function eventColor(event: ModelEvent) {
  if (event.severity === 'error' || event.status === 'error') return 'error'
  if (event.severity === 'warning' || event.status === 'skipped' || event.status === 'inactive') {
    return 'warning'
  }
  return 'success'
}
</script>

<template>
  <VCard class="chart-card model-events-timeline-card">
    <div class="chart-card__header">
      <div>
        <p class="eyebrow">events</p>
        <h2>Model events</h2>
      </div>
      <VChip size="small" variant="tonal">15 latest</VChip>
    </div>

    <div v-if="visibleEvents.length === 0" class="empty-chart">
      No model event yet
    </div>

    <ol v-else class="model-events-timeline-card__list" aria-label="Latest model events">
      <li
        v-for="event in visibleEvents"
        :key="event.id"
        class="model-events-timeline-card__item"
      >
        <button
          class="model-events-timeline-card__button"
          type="button"
          :aria-label="`Open events for ${event.provider_id}/${event.model_id}`"
          @click="emit('openEvents', modelIdentity(event))"
        >
          <span class="model-events-timeline-card__rail" aria-hidden="true">
            <span class="model-events-timeline-card__dot" :class="`model-events-timeline-card__dot--${eventColor(event)}`" />
          </span>
          <span class="model-events-timeline-card__content">
            <span class="model-events-timeline-card__meta">
              <time :datetime="event.observed_at">{{ formatTimelineDateTime(event.observed_at) }}</time>
              <VChip size="x-small" :color="eventColor(event)" variant="tonal">
                {{ event.event_type }}
              </VChip>
              <span>{{ event.source }}</span>
            </span>
            <span class="model-events-timeline-card__model model-name">{{ event.provider_id }}/{{ event.model_id }}</span>
            <strong>{{ event.title }}</strong>
            <span>{{ event.message }}</span>
          </span>
        </button>
      </li>
    </ol>
  </VCard>
</template>

<style scoped>
.model-events-timeline-card__list {
  height: 260px;
  margin: 0;
  padding: 0 4px 0 0;
  overflow-y: auto;
  list-style: none;
}

.model-events-timeline-card__item {
  position: relative;
}

.model-events-timeline-card__button {
  display: grid;
  width: 100%;
  grid-template-columns: 18px minmax(0, 1fr);
  gap: 10px;
  padding: 0 0 14px;
  border: 0;
  color: inherit;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.model-events-timeline-card__button:hover strong,
.model-events-timeline-card__button:focus-visible strong {
  color: var(--color-status-ok);
}

.model-events-timeline-card__button:focus-visible {
  border-radius: 8px;
  outline: 2px solid var(--color-status-ok);
  outline-offset: 3px;
}

.model-events-timeline-card__rail {
  position: relative;
  display: flex;
  justify-content: center;
  min-height: 100%;
}

.model-events-timeline-card__rail::after {
  position: absolute;
  top: 14px;
  bottom: -2px;
  width: 1px;
  background: var(--border);
  content: "";
}

.model-events-timeline-card__item:last-child .model-events-timeline-card__rail::after {
  display: none;
}

.model-events-timeline-card__dot {
  position: relative;
  z-index: 1;
  width: 10px;
  height: 10px;
  margin-top: 5px;
  border: 2px solid var(--bg-card);
  border-radius: 999px;
  background: var(--text-muted);
  box-shadow: 0 0 0 1px var(--border);
}

.model-events-timeline-card__dot--success {
  background: #10a37f;
}

.model-events-timeline-card__dot--warning {
  background: #b7791f;
}

.model-events-timeline-card__dot--error {
  background: #dc2626;
}

.model-events-timeline-card__content,
.model-events-timeline-card__meta {
  display: flex;
  min-width: 0;
}

.model-events-timeline-card__content {
  flex-direction: column;
  gap: 4px;
}

.model-events-timeline-card__meta {
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  color: var(--text-muted);
  font-size: 0.75rem;
}

.model-events-timeline-card__model {
  max-width: 100%;
}

.model-events-timeline-card__content strong,
.model-events-timeline-card__content span {
  overflow-wrap: anywhere;
}

.model-events-timeline-card__content strong {
  color: var(--text-primary);
  font-size: 0.875rem;
  line-height: 1.25;
  transition: color var(--transition);
}

.model-events-timeline-card__content > span:last-child {
  color: var(--text-secondary);
  font-size: 0.8125rem;
  line-height: 1.35;
}
</style>
