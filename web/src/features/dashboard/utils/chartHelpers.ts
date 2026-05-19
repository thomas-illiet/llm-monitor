import type { ConfiguredChart } from '@/types'

/** Entry rendered in the configured chart grid. */
export type ChartGridEntry =
  | { kind: 'chart', key: string, chart: ConfiguredChart }
  | { kind: 'model-status', key: string }
  | { kind: 'model-capability', key: string }
  | { kind: 'model-events', key: string }

export interface DashboardChartTheme {
  axis: string
  grid: string
  legend: string
  tooltipBg: string
  tooltipText: string
}

const palette = ['#10a37f', '#2563eb', '#b7791f', '#7c3aed', '#dc2626', '#4b5563']

/** Returns Chart.js colors for the active dashboard theme. */
export function chartTheme(isDark: boolean): DashboardChartTheme {
  return {
    axis: isDark ? '#c5c5d2' : '#6b7280',
    grid: isDark ? '#3d3d42' : '#ecebe7',
    legend: isDark ? '#ececf1' : '#4b5563',
    tooltipBg: isDark ? '#17181a' : '#202123',
    tooltipText: '#ffffff'
  }
}

/** Maps the API chart shape to the vue-chartjs dataset shape. */
export function chartData(chart: ConfiguredChart): any {
  const isLine = chart.type === 'line'
  return {
    labels: chart.labels,
    datasets: chart.datasets.map((dataset, index: number) => ({
      label: dataset.label,
      data: dataset.data,
      borderColor: palette[index % palette.length],
      backgroundColor: `${palette[index % palette.length]}33`,
      borderWidth: isLine ? 2 : 1,
      borderRadius: isLine ? 0 : 4,
      tension: 0,
      spanGaps: isLine,
      pointRadius: isLine ? 1.5 : 0
    }))
  }
}

/** Creates consistent Chart.js options for line and bar dashboard charts. */
export function chartOptions(chart: ConfiguredChart, colors: DashboardChartTheme): any {
  const stacked = chart.type === 'stacked-bar'
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' },
    plugins: {
      legend: {
        display: chart.datasets.length > 1,
        labels: {
          boxWidth: 10,
          boxHeight: 10,
          color: colors.legend
        }
      },
      tooltip: {
        backgroundColor: colors.tooltipBg,
        titleColor: colors.tooltipText,
        bodyColor: colors.tooltipText,
        callbacks: {
          label(context: any) {
            const parsedY = context.parsed?.y
            if (typeof parsedY !== 'number' || Number.isNaN(parsedY)) {
              return `${context.dataset.label}: no sample`
            }
            const value = Math.round(parsedY * 100) / 100
            return `${context.dataset.label}: ${value}`
          }
        }
      }
    },
    scales: {
      x: {
        stacked,
        grid: { display: false },
        ticks: { color: colors.axis, maxRotation: 0, autoSkip: true, maxTicksLimit: 8 }
      },
      y: {
        stacked,
        beginAtZero: true,
        grid: { color: colors.grid },
        ticks: { color: colors.axis }
      }
    }
  }
}

/** Reports whether a configured chart represents target HTTP latency. */
export function isHttpLatencyChart(chart: ConfiguredChart) {
  const id = chart.id.toLowerCase()
  const metric = chart.metric.toLowerCase()
  return metric === 'http_latency_ms' || id.includes('http-latency')
}

/** Interleaves inventory charts after the first HTTP latency chart. */
export function chartGridEntries(charts: ConfiguredChart[]): ChartGridEntry[] {
  const entries: ChartGridEntry[] = []
  let insertedInventoryCards = false

  for (const chart of charts) {
    entries.push({ kind: 'chart', key: `chart-${chart.id}`, chart })
    if (!insertedInventoryCards && isHttpLatencyChart(chart)) {
      entries.push(...inventoryChartEntries())
      insertedInventoryCards = true
    }
  }

  if (!insertedInventoryCards) {
    entries.push(...inventoryChartEntries())
  }

  return entries
}

/** Returns current-inventory chart cards in their dashboard display order. */
function inventoryChartEntries(): ChartGridEntry[] {
  return [
    { kind: 'model-status', key: 'model-status-card' },
    { kind: 'model-capability', key: 'model-capability-card' },
    { kind: 'model-events', key: 'model-events-card' }
  ]
}
