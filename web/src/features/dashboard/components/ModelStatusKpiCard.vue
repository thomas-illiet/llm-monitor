<script setup lang="ts">
import { computed } from 'vue'
import { Boxes } from '@lucide/vue'
import type { ConfiguredChart, StatusSummary } from '@/types'

const props = defineProps<{
  status: StatusSummary
  modelStatusHistory: ConfiguredChart
}>()

interface ModelStatusSeries {
  key: string
  label: string
  value: number
  color: string
  path: string
}

interface TimelineTick {
  key: string
  label: string
}

const modelStatusColors: Record<string, string> = {
  active: '#10a37f',
  missing: '#b42318',
  skipped: '#b7791f'
}
const modelStatusOrder = ['active', 'missing', 'skipped']
const maxTimelineTicks = 5

/** Normalizes status labels from chart datasets for lookup. */
function normalizeStatusLabel(label: string) {
  return label.trim().toLowerCase()
}

/** Converts normalized status labels into compact display labels. */
function displayStatusLabel(label: string) {
  const normalized = normalizeStatusLabel(label)
  return `${normalized.charAt(0).toUpperCase()}${normalized.slice(1)}`
}

/** Returns the latest value in a chart series. */
function lastValue(values: number[]) {
  return values.length > 0 ? values[values.length - 1] : 0
}

/** Prefers the live status count over lagging chart samples. */
function currentStatusValue(key: string, values: number[]) {
  switch (key) {
    case 'active':
      return props.status.active_models
    case 'missing':
      return props.status.missing_models
    case 'skipped':
      return props.status.skipped_models
    default:
      return Math.round(lastValue(values))
  }
}

/** Builds the SVG path for a small inline trend line. */
function sparklinePath(values: number[], maxValue: number) {
  const width = 180
  const height = 44
  const padding = 3
  const sanitized = values.map(value => Number.isFinite(value) ? value : 0)
  if (sanitized.length === 0) return ''
  const usableWidth = width - padding * 2
  const usableHeight = height - padding * 2
  const scaleY = (value: number) => padding + usableHeight - (Math.max(value, 0) / maxValue) * usableHeight

  if (sanitized.length === 1) {
    const y = scaleY(sanitized[0])
    return `M ${padding} ${y} L ${width - padding} ${y}`
  }

  return sanitized.map((value, index) => {
    const x = padding + (index / (sanitized.length - 1)) * usableWidth
    const y = scaleY(value)
    return `${index === 0 ? 'M' : 'L'} ${x} ${y}`
  }).join(' ')
}

/** Selects evenly spaced labels for the compact timeline axis. */
function timelineTicks(labels: string[]): TimelineTick[] {
  if (labels.length === 0) return []
  if (labels.length <= maxTimelineTicks) {
    return labels.map((label, index) => ({ key: `${index}-${label}`, label }))
  }

  const lastIndex = labels.length - 1
  const indexes = new Set<number>()
  for (let step = 0; step < maxTimelineTicks; step++) {
    indexes.add(Math.round((step / (maxTimelineTicks - 1)) * lastIndex))
  }

  return [...indexes].sort((left, right) => left - right).map(index => ({
    key: `${index}-${labels[index]}`,
    label: labels[index]
  }))
}

const modelCount = computed(() => props.status.active_models + props.status.missing_models)
const modelDetail = computed(() => {
  return `${props.status.active_models} active · ${props.status.missing_models} missing · ${props.status.skipped_models} skipped`
})
const modelAccent = computed(() => props.status.missing_models > 0 ? '#b42318' : '#10a37f')
const modelBadge = computed(() => props.status.missing_models > 0 ? 'some' : 'none')

const modelStatusSeries = computed<ModelStatusSeries[]>(() => {
  const datasets = props.modelStatusHistory.datasets ?? []
  const maxValue = Math.max(1, ...datasets.flatMap(dataset => dataset.data))
  return datasets
    .map(dataset => {
      const key = normalizeStatusLabel(dataset.label)
      return {
        key,
        label: displayStatusLabel(dataset.label),
        value: currentStatusValue(key, dataset.data),
        color: modelStatusColors[key] ?? '#6b7280',
        path: sparklinePath(dataset.data, maxValue)
      }
    })
    .sort((left, right) => {
      const leftIndex = modelStatusOrder.indexOf(left.key)
      const rightIndex = modelStatusOrder.indexOf(right.key)
      return (leftIndex === -1 ? modelStatusOrder.length : leftIndex) -
        (rightIndex === -1 ? modelStatusOrder.length : rightIndex)
    })
})

const modelTimelineTicks = computed(() => timelineTicks(props.modelStatusHistory.labels ?? []))
</script>

<template>
  <VCard class="chart-card kpi-card model-status-card">
    <div class="kpi-card__header">
      <div class="kpi-card__icon" :style="{ color: modelAccent }">
        <Boxes :size="18" />
      </div>
      <p class="kpi-card__label">Models</p>
    </div>
    <strong :style="{ color: modelAccent }">{{ modelCount }}</strong>
    <span>{{ modelDetail }}</span>
    <div class="kpi-badge" :class="`kpi-badge--${modelBadge}`">
      {{ modelBadge }}
    </div>
    <div v-if="modelStatusSeries.length > 0" class="model-status-trend">
      <svg
        class="model-status-trend__chart"
        viewBox="0 0 180 44"
        preserveAspectRatio="none"
        aria-label="Model count trend by status"
      >
        <path
          v-for="series in modelStatusSeries"
          :key="series.key"
          class="model-status-trend__line"
          :d="series.path"
          :stroke="series.color"
        />
      </svg>
      <div
        v-if="modelTimelineTicks.length > 0"
        class="model-status-trend__timeline"
        aria-label="Model status timeline"
      >
        <span
          v-for="tick in modelTimelineTicks"
          :key="tick.key"
          class="model-status-trend__timeline-tick"
        >
          {{ tick.label }}
        </span>
      </div>
      <div class="model-status-trend__legend">
        <span
          v-for="series in modelStatusSeries"
          :key="`${series.key}-legend`"
          class="model-status-trend__legend-item"
        >
          <i :style="{ backgroundColor: series.color }" />
          {{ series.label }} {{ series.value }}
        </span>
      </div>
    </div>
  </VCard>
</template>
