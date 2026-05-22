import type { ConfiguredChart, KpiSummary } from '@/types'
import {
  formatCompactDecimal,
  formatCompactNumber,
  formatDecimal,
  formatDurationMs,
  normalizeZero
} from '@/features/dashboard/utils/formatters'

type ComparisonDirection = 'higher' | 'lower' | 'neutral'
type KpiFormat = 'compact' | 'duration' | 'integer' | 'percent' | 'rate'

interface ComparisonMetricSpec {
  direction: ComparisonDirection
  format: KpiFormat
  key: keyof KpiSummary
  label: string
}

interface ComparisonDelta {
  label: string
  percentageLabel: string
  percentageValue: number | null
  tone: 'negative' | 'neutral' | 'positive'
  value: number
}

export interface ComparisonMetric {
  comparedLabel: string
  comparedValue: number
  delta: ComparisonDelta
  key: keyof KpiSummary
  label: string
  referenceLabel: string
  referenceValue: number
}

export interface ComparisonChart extends ConfiguredChart {
  compared_model_id: string
  reference_model_id: string
}

const chatMetricSpecs: ComparisonMetricSpec[] = [
  { key: 'ttft_p99_ms', label: 'TTFT p99', format: 'duration', direction: 'lower' },
  { key: 'itl_p99_ms', label: 'ITL p99', format: 'duration', direction: 'lower' },
  { key: 'request_latency_p99_ms', label: 'Request p99', format: 'duration', direction: 'lower' },
  { key: 'output_tokens_per_second', label: 'Output rate', format: 'rate', direction: 'higher' },
  { key: 'success_rate', label: 'Success rate', format: 'percent', direction: 'higher' },
  { key: 'slo_violation_count', label: 'SLO violations', format: 'integer', direction: 'lower' },
  { key: 'error_count', label: 'Errors', format: 'integer', direction: 'lower' },
  { key: 'total_runs', label: 'Runs', format: 'integer', direction: 'neutral' }
]

const embeddingMetricSpecs: ComparisonMetricSpec[] = [
  { key: 'request_latency_p99_ms', label: 'Request p99', format: 'duration', direction: 'lower' },
  { key: 'success_rate', label: 'Success rate', format: 'percent', direction: 'higher' },
  { key: 'input_tokens', label: 'Input tokens', format: 'compact', direction: 'neutral' },
  { key: 'slo_violation_count', label: 'SLO violations', format: 'integer', direction: 'lower' },
  { key: 'error_count', label: 'Errors', format: 'integer', direction: 'lower' },
  { key: 'total_runs', label: 'Runs', format: 'integer', direction: 'neutral' }
]

/** Builds the KPI rows that can be compared for one model capability. */
export function comparisonMetrics(reference: KpiSummary, compared: KpiSummary, capability: string): ComparisonMetric[] {
  return metricSpecs(capability).map(spec => {
    const referenceValue = Number(reference[spec.key] ?? 0)
    const comparedValue = Number(compared[spec.key] ?? 0)
    return {
      comparedLabel: formatValue(comparedValue, spec.format),
      comparedValue,
      delta: comparisonDelta(referenceValue, comparedValue, spec.format, spec.direction),
      key: spec.key,
      label: spec.label,
      referenceLabel: formatValue(referenceValue, spec.format),
      referenceValue
    }
  })
}

/** Combines matching chart series from two model dashboard payloads. */
export function comparisonCharts(
  referenceCharts: ConfiguredChart[],
  comparedCharts: ConfiguredChart[],
  referenceModelId: string,
  comparedModelId: string
): ComparisonChart[] {
  return referenceCharts.flatMap(referenceChart => {
    const comparedChart = comparedCharts.find(chart => chart.id === referenceChart.id && chart.metric === referenceChart.metric)
    if (!comparedChart) return []

    const labels = mergedLabels(referenceChart.labels, comparedChart.labels)
    return [{
      compared_model_id: comparedModelId,
      datasets: [
        ...chartDatasets(referenceChart, labels, referenceModelId),
        ...chartDatasets(comparedChart, labels, comparedModelId)
      ],
      error: [referenceChart.error, comparedChart.error].filter(Boolean).join(' / ') || undefined,
      id: `comparison-${referenceChart.id}`,
      labels,
      metric: referenceChart.metric,
      reference_model_id: referenceModelId,
      title: referenceChart.title,
      type: referenceChart.type === 'stacked-bar' ? 'bar' : referenceChart.type
    }]
  })
}

function metricSpecs(capability: string) {
  return capability === 'embedding' ? embeddingMetricSpecs : chatMetricSpecs
}

function comparisonDelta(referenceValue: number, comparedValue: number, format: KpiFormat, direction: ComparisonDirection): ComparisonDelta {
  const value = normalizeZero(comparedValue - referenceValue)
  const percentageValue = referenceValue === 0 ? null : normalizeZero(value / referenceValue)
  return {
    label: formatDelta(value, format),
    percentageLabel: percentageValue === null ? 'n/a' : `${formatSignedNumber(percentageValue * 100, 1)}%`,
    percentageValue,
    tone: deltaTone(value, direction),
    value
  }
}

function deltaTone(delta: number, direction: ComparisonDirection): ComparisonDelta['tone'] {
  if (delta === 0 || direction === 'neutral') return 'neutral'
  if (direction === 'higher') return delta > 0 ? 'positive' : 'negative'
  return delta < 0 ? 'positive' : 'negative'
}

function chartDatasets(chart: ConfiguredChart, labels: string[], modelId: string) {
  return chart.datasets.map(dataset => ({
    label: `${modelId} - ${dataset.label}`,
    data: labels.map(label => {
      const index = chart.labels.indexOf(label)
      return index === -1 ? null : dataset.data[index]
    })
  }))
}

function mergedLabels(referenceLabels: string[], comparedLabels: string[]) {
  return [...referenceLabels, ...comparedLabels.filter(label => !referenceLabels.includes(label))]
}

function formatValue(value: number, format: KpiFormat) {
  switch (format) {
    case 'duration':
      return formatDurationMs(value)
    case 'percent':
      return `${formatDecimal(value * 100, 1)}%`
    case 'rate':
      return `${formatDecimal(value, 1)} tok/s`
    case 'integer':
      return formatDecimal(Math.round(value), 0)
    case 'compact':
      return formatCompactNumber(value)
  }
}

function formatDelta(value: number, format: KpiFormat) {
  if (format === 'percent') return `${formatSignedNumber(value * 100, 1)} pp`
  if (format === 'duration') return `${value > 0 ? '+' : value < 0 ? '-' : ''}${formatDurationMs(Math.abs(value))}`
  if (format === 'rate') return `${formatSignedNumber(value, 1)} tok/s`
  if (format === 'integer') return formatSignedNumber(Math.round(value), 0)
  return formatSignedCompact(value)
}

function formatSignedCompact(value: number) {
  const sign = value > 0 ? '+' : ''
  return `${sign}${formatCompactDecimal(value)}`
}

function formatSignedNumber(value: number, maximumFractionDigits: number) {
  const normalized = normalizeZero(value)
  const sign = normalized > 0 ? '+' : ''
  return `${sign}${formatDecimal(normalized, maximumFractionDigits)}`
}
