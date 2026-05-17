<script setup lang="ts">
import { computed, type Component } from 'vue'
import { Activity, AlertTriangle, Database, FileText, Gauge, Hash, Timer, TrendingUp, Zap } from '@lucide/vue'
import type { EmbeddingRecentRun, KpiSummary, RecentRun, SloThresholds } from '@/types'

const props = defineProps<{
  capability?: string
  kpis: KpiSummary
  runs?: RecentRun[]
  slo: SloThresholds
}>()

interface KpiCard {
  label: string
  value: string
  detail: string
  badge: string | null
  accent: string
  icon: Component
  progress?: number
}

/** Formats millisecond values for compact KPI cards. */
function ms(value: number) {
  return `${Math.round(value)} ms`
}

/** Formats a ratio as a one-decimal percentage. */
function percent(value: number) {
  return `${Math.round(value * 1000) / 10}%`
}

/** Formats large counters using locale-aware compact notation. */
function compact(value: number) {
  return new Intl.NumberFormat(undefined, { notation: 'compact' }).format(value)
}

/** Formats generated-token throughput. */
function rate(value: number) {
  return `${Math.round(value * 10) / 10} tok/s`
}

/** Formats byte counts without implying precision we do not capture. */
function bytes(value?: number) {
  if (value === undefined) return 'Unknown'
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 1,
    notation: value >= 100_000 ? 'compact' : 'standard'
  }).format(value) + ' B'
}

/** Chooses the accent color for a threshold-backed KPI value. */
function thresholdAccent(value: number, threshold: number): string {
  if (value === 0) return '#6b7280'
  if (value <= threshold) return '#0f8f6f'
  if (value <= threshold * 1.5) return '#b7791f'
  return '#b42318'
}

/** Chooses the status badge for a threshold-backed KPI value. */
function thresholdBadge(value: number, threshold: number): string {
  if (value === 0) return 'none'
  if (value <= threshold) return 'fast'
  if (value <= threshold * 1.5) return 'ok'
  return 'slow'
}

const successPercent = computed(() => Math.round(props.kpis.success_rate * 100))

const latestEmbeddingRun = computed<EmbeddingRecentRun | null>(() => {
  return props.runs?.find((run): run is EmbeddingRecentRun => run.kind === 'embedding') ?? null
})

const latestEmbeddingRunWithDimensions = computed<EmbeddingRecentRun | null>(() => {
  return props.runs?.find((run): run is EmbeddingRecentRun => {
    return run.kind === 'embedding' && run.vector_dimensions !== undefined
  }) ?? null
})

const cards = computed<KpiCard[]>(() => {
  if (props.capability === 'embedding') {
    const latestRun = latestEmbeddingRun.value
    const latestDimensionsRun = latestEmbeddingRunWithDimensions.value
    const issueCount = props.kpis.slo_violation_count
    return [
      {
        label: 'Request p99',
        value: ms(props.kpis.request_latency_p99_ms),
        detail: `SLO ${ms(props.slo.request_latency_p99_ms)} · p95 ${ms(props.kpis.request_latency_p95_ms)}`,
        badge: thresholdBadge(props.kpis.request_latency_p99_ms, props.slo.request_latency_p99_ms),
        accent: thresholdAccent(props.kpis.request_latency_p99_ms, props.slo.request_latency_p99_ms),
        icon: Zap
      },
      {
        label: 'Success rate',
        value: percent(props.kpis.success_rate),
        detail: `${props.kpis.total_runs} embedding runs`,
        badge: null,
        accent: '#10a37f',
        icon: Gauge,
        progress: successPercent.value
      },
      {
        label: 'Input tokens',
        value: compact(props.kpis.input_tokens),
        detail: 'Embedding fixture tokens',
        badge: null,
        accent: '#2563eb',
        icon: Database
      },
      {
        label: 'Vector dimensions',
        value: latestDimensionsRun?.vector_dimensions === undefined ? 'Unknown' : compact(latestDimensionsRun.vector_dimensions),
        detail: 'Latest recorded embedding vector',
        badge: null,
        accent: '#6d5bd0',
        icon: Hash
      },
      {
        label: 'Fixture size',
        value: bytes(latestRun?.fixture_bytes),
        detail: latestRun?.fixture_path ?? 'Fixture path not recorded',
        badge: null,
        accent: '#0f766e',
        icon: FileText
      },
      {
        label: 'Issues',
        value: String(issueCount),
        detail: `${props.kpis.slo_violation_count} SLO violations · ${props.kpis.error_count} errors`,
        badge: issueCount === 0 ? 'none' : 'some',
        accent: issueCount === 0 ? '#0f8f6f' : '#b42318',
        icon: AlertTriangle
      }
    ]
  }

  return [
    {
      label: 'TTFT p99',
      value: ms(props.kpis.ttft_p99_ms),
      detail: `SLO ${ms(props.slo.ttft_p99_ms)} · p50 ${ms(props.kpis.ttft_p50_ms)}`,
      badge: thresholdBadge(props.kpis.ttft_p99_ms, props.slo.ttft_p99_ms),
      accent: thresholdAccent(props.kpis.ttft_p99_ms, props.slo.ttft_p99_ms),
      icon: Timer
    },
    {
      label: 'ITL p99',
      value: ms(props.kpis.itl_p99_ms),
      detail: `SLO ${ms(props.slo.itl_p99_ms)} · p50 ${ms(props.kpis.itl_p50_ms)}`,
      badge: thresholdBadge(props.kpis.itl_p99_ms, props.slo.itl_p99_ms),
      accent: thresholdAccent(props.kpis.itl_p99_ms, props.slo.itl_p99_ms),
      icon: Activity
    },
    {
      label: 'Request p99',
      value: ms(props.kpis.request_latency_p99_ms),
      detail: `SLO ${ms(props.slo.request_latency_p99_ms)} · p95 ${ms(props.kpis.request_latency_p95_ms)}`,
      badge: thresholdBadge(props.kpis.request_latency_p99_ms, props.slo.request_latency_p99_ms),
      accent: thresholdAccent(props.kpis.request_latency_p99_ms, props.slo.request_latency_p99_ms),
      icon: Zap
    },
    {
      label: 'Output rate',
      value: rate(props.kpis.output_tokens_per_second),
      detail: `${compact(props.kpis.output_tokens)} output tokens`,
      badge: null,
      accent: '#2563eb',
      icon: TrendingUp
    },
    {
      label: 'Success rate',
      value: percent(props.kpis.success_rate),
      detail: `${props.kpis.total_runs} runs`,
      badge: null,
      accent: '#10a37f',
      icon: Gauge,
      progress: successPercent.value
    },
    {
      label: 'SLO violations',
      value: String(props.kpis.slo_violation_count),
      detail: `${props.kpis.degraded_models} degraded models · ${props.kpis.error_count} errors`,
      badge: props.kpis.slo_violation_count === 0 ? 'none' : 'some',
      accent: props.kpis.slo_violation_count === 0 ? '#0f8f6f' : '#b42318',
      icon: AlertTriangle
    }
  ]
})
</script>

<template>
  <section class="kpi-grid">
    <VCard
      v-for="card in cards"
      :key="card.label"
      class="kpi-card"
    >
      <div class="kpi-card__header">
        <div class="kpi-card__icon" :style="{ color: card.accent }">
          <component :is="card.icon" :size="18" />
        </div>
        <p class="kpi-card__label">{{ card.label }}</p>
      </div>
      <strong :style="{ color: card.accent }">{{ card.value }}</strong>
      <span>{{ card.detail }}</span>
      <div v-if="card.badge" class="kpi-badge" :class="`kpi-badge--${card.badge}`">
        {{ card.badge }}
      </div>
      <div v-if="card.progress !== undefined" class="kpi-progress">
        <div class="kpi-progress__bar" :style="{ width: `${card.progress}%` }" />
      </div>
    </VCard>
  </section>
</template>
