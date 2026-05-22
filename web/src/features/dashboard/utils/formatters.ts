const mediumDateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short'
})

const preciseDateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'medium'
})

const timelineDateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit'
})

const compactNumberFormatter = new Intl.NumberFormat(undefined, { notation: 'compact' })
const compactDecimalFormatter = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 1,
  notation: 'compact'
})
const integerFormatter = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 })

/** Formats an optional timestamp for table and summary rows. */
export function formatMediumDateTime(value?: string) {
  if (!value) return 'n/a'
  return mediumDateTimeFormatter.format(new Date(value))
}

/** Formats a required timestamp with seconds for dense event/run tables. */
export function formatPreciseDateTime(value: string) {
  return preciseDateTimeFormatter.format(new Date(value))
}

/** Formats a required timestamp for compact timeline cards. */
export function formatTimelineDateTime(value: string) {
  return timelineDateTimeFormatter.format(new Date(value))
}

/** Formats a number with locale-aware compact notation. */
export function formatCompactNumber(value: number) {
  return compactNumberFormatter.format(value)
}

/** Formats compact values with one decimal when the locale needs it. */
export function formatCompactDecimal(value: number) {
  return compactDecimalFormatter.format(value)
}

/** Formats a signed or unsigned number with a fixed maximum precision. */
export function formatDecimal(value: number, maximumFractionDigits: number) {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits,
    minimumFractionDigits: maximumFractionDigits
  }).format(normalizeZero(value))
}

/** Formats millisecond values, switching to seconds at one second. */
export function formatDurationMs(value: number) {
  const milliseconds = Math.round(value)
  if (milliseconds < 1000) return `${milliseconds} ms`
  return `${formatDecimal(milliseconds / 1000, 1)} s`
}

/** Formats ratios as percentages. */
export function formatPercent(value: number, maximumFractionDigits = 1) {
  return `${formatDecimal(value * 100, maximumFractionDigits)}%`
}

/** Formats token throughput. */
export function formatTokenRate(value: number) {
  return `${formatDecimal(value, 1)} tok/s`
}

/** Formats byte counts without implying precision that probes do not capture. */
export function formatBytes(value?: number, fallback = 'Unknown') {
  if (value === undefined) return fallback
  const formatter = value >= 100_000 ? compactDecimalFormatter : integerFormatter
  return `${formatter.format(value)} B`
}

/** Collapses tiny floating point noise to zero before display. */
export function normalizeZero(value: number) {
  return Math.abs(value) < 0.000001 ? 0 : value
}
