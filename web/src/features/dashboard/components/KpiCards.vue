<script setup lang="ts">
import { computed, type Component } from 'vue'
import { Activity, AlertTriangle, Database, FileText, Gauge, Hash, Timer, TrendingUp, Zap } from '@lucide/vue'
import {
  formatBytes,
  formatCompactNumber,
  formatDurationMs,
  formatPercent,
  formatTokenRate
} from '@/features/dashboard/utils/formatters'
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
  accent: string
  icon: Component
}

/** Chooses the accent color for a threshold-backed KPI value. */
function thresholdAccent(value: number, threshold: number): string {
  if (value === 0) return '#6b7280'
  if (value <= threshold) return '#0f8f6f'
  if (value <= threshold * 1.5) return '#b7791f'
  return '#b42318'
}

const latestEmbeddingRun = computed<EmbeddingRecentRun | null>(() => {
  return props.runs?.find((run): run is EmbeddingRecentRun => run.capability === 'embedding') ?? null
})

const latestEmbeddingRunWithDimensions = computed<EmbeddingRecentRun | null>(() => {
  return props.runs?.find((run): run is EmbeddingRecentRun => {
    return run.capability === 'embedding' && run.vector_dimensions !== undefined
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
        value: formatDurationMs(props.kpis.request_latency_p99_ms),
        detail: `SLO ${formatDurationMs(props.slo.request_latency_p99_ms)} · p95 ${formatDurationMs(props.kpis.request_latency_p95_ms)}`,
        accent: thresholdAccent(props.kpis.request_latency_p99_ms, props.slo.request_latency_p99_ms),
        icon: Zap
      },
      {
        label: 'Success rate',
        value: formatPercent(props.kpis.success_rate),
        detail: `${props.kpis.total_runs} embedding runs`,
        accent: '#10a37f',
        icon: Gauge
      },
      {
        label: 'Input tokens',
        value: formatCompactNumber(props.kpis.input_tokens),
        detail: 'Fixture tokens',
        accent: '#2563eb',
        icon: Database
      },
      {
        label: 'Vector dimensions',
        value: latestDimensionsRun?.vector_dimensions === undefined ? 'Unknown' : formatCompactNumber(latestDimensionsRun.vector_dimensions),
        detail: 'Latest recorded vector',
        accent: '#6d5bd0',
        icon: Hash
      },
      {
        label: 'Fixture size',
        value: formatBytes(latestRun?.fixture_bytes),
        detail: 'Fixture bytes',
        accent: '#0f766e',
        icon: FileText
      },
      {
        label: 'Issues',
        value: String(issueCount),
        detail: `${props.kpis.degraded_models} degraded · ${props.kpis.error_count} errors`,
        accent: issueCount === 0 ? '#0f8f6f' : '#b42318',
        icon: AlertTriangle
      }
    ]
  }

  return [
    {
      label: 'TTFT p99',
      value: formatDurationMs(props.kpis.ttft_p99_ms),
      detail: `SLO ${formatDurationMs(props.slo.ttft_p99_ms)} · p50 ${formatDurationMs(props.kpis.ttft_p50_ms)}`,
      accent: thresholdAccent(props.kpis.ttft_p99_ms, props.slo.ttft_p99_ms),
      icon: Timer
    },
    {
      label: 'ITL p99',
      value: formatDurationMs(props.kpis.itl_p99_ms),
      detail: `SLO ${formatDurationMs(props.slo.itl_p99_ms)} · p50 ${formatDurationMs(props.kpis.itl_p50_ms)}`,
      accent: thresholdAccent(props.kpis.itl_p99_ms, props.slo.itl_p99_ms),
      icon: Activity
    },
    {
      label: 'Request p99',
      value: formatDurationMs(props.kpis.request_latency_p99_ms),
      detail: `SLO ${formatDurationMs(props.slo.request_latency_p99_ms)} · p95 ${formatDurationMs(props.kpis.request_latency_p95_ms)}`,
      accent: thresholdAccent(props.kpis.request_latency_p99_ms, props.slo.request_latency_p99_ms),
      icon: Zap
    },
    {
      label: 'Output rate',
      value: formatTokenRate(props.kpis.output_tokens_per_second),
      detail: `${formatCompactNumber(props.kpis.output_tokens)} output tokens`,
      accent: '#2563eb',
      icon: TrendingUp
    },
    {
      label: 'Success rate',
      value: formatPercent(props.kpis.success_rate),
      detail: `${props.kpis.total_runs} runs`,
      accent: '#10a37f',
      icon: Gauge
    },
    {
      label: 'SLO violations',
      value: String(props.kpis.slo_violation_count),
      detail: `${props.kpis.degraded_models} degraded · ${props.kpis.error_count} errors`,
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
    </VCard>
  </section>
</template>
