<script setup lang="ts">
import { computed } from 'vue'
import { Boxes, RadioTower } from '@lucide/vue'
import type { CheckRecord, StatusSummary } from '@/types'

const props = defineProps<{
  status?: StatusSummary
  http?: CheckRecord
}>()

/** Formats the latest HTTP latency for the public status card. */
function formatLatency(check?: CheckRecord) {
  if (!check) return 'n/a'
  return `${Math.round(check.latency_ms)} ms`
}

const modelSummary = computed(() => {
  if (!props.status) return 'No model snapshot yet'
  return `${props.status.active_models} active / ${props.status.inactive_models} inactive / ${props.status.skipped_models} skipped`
})

const cards = computed(() => [
  {
    title: 'LLM HTTP check',
    icon: RadioTower,
    ok: props.http?.ok ?? false,
    status: props.http?.ok ? 'Reachable' : 'Degraded',
    latency: formatLatency(props.http),
    detail: props.http ? props.http.error || `Status ${props.http.status_code}` : 'No HTTP check yet'
  },
  {
    title: 'Model availability',
    icon: Boxes,
    ok: props.status?.inactive_models === 0,
    status: props.status?.inactive_models === 0 ? 'Complete' : 'Attention required',
    latency: `${props.status?.active_models ?? 0} models`,
    detail: modelSummary.value
  }
])
</script>

<template>
  <section class="status-panel">
    <VCard v-for="card in cards" :key="card.title" class="service-card">
      <div class="service-card__icon" :class="{ 'service-card__icon--ok': card.ok }">
        <component :is="card.icon" :size="18" />
      </div>
      <div class="service-card__body">
        <p class="service-card__title">{{ card.title }}</p>
        <div class="service-card__line">
          <strong>{{ card.status }}</strong>
          <span>{{ card.latency }}</span>
        </div>
        <p class="service-card__detail">{{ card.detail }}</p>
      </div>
    </VCard>
  </section>
</template>
