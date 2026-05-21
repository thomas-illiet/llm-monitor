<script setup lang="ts">
import { computed, shallowRef, onMounted, onUnmounted } from 'vue'
import { Activity, PlayCircle, RefreshCw, Moon, Sun } from '@lucide/vue'
import type { StatusSummary } from '@/types'

const props = defineProps<{
  status?: StatusSummary
  generatedAt?: string
  siteName: string
  loading: boolean
  checking: boolean
  isDark: boolean
}>()

const emit = defineEmits<{
  refresh: []
  'run-checks': []
  'toggle-theme': []
}>()

const taglines = [
  "Watching your models so you don't have to",
  'Beep boop - all neurons firing',
  'Your AI babysitter on duty',
  'Latency obsessed, token possessed',
  'If it breaks, we scream first',
  'Counting tokens since forever',
  'No hallucinations here... probably',
]
const taglineIndex = shallowRef(0)
const taglineFading = shallowRef(false)

let taglineTimer: ReturnType<typeof setInterval> | null = null
let fadeTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  taglineTimer = setInterval(() => {
    taglineFading.value = true
    fadeTimer = setTimeout(() => {
      taglineIndex.value = (taglineIndex.value + 1) % taglines.length
      taglineFading.value = false
    }, 300)
  }, 4000)
})

onUnmounted(() => {
  if (taglineTimer) clearInterval(taglineTimer)
  if (fadeTimer) clearTimeout(fadeTimer)
})

const label = computed(() => {
  if (!props.status) return 'Pending telemetry'
  return props.status.ok ? 'Operational' : 'Attention required'
})

const generatedLabel = computed(() => {
  if (!props.generatedAt) return 'No snapshot yet'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium'
  }).format(new Date(props.generatedAt))
})
</script>

<template>
  <header class="status-header">
    <div class="status-title-row">
      <div class="status-mark" :class="{ 'status-mark--ok': status?.ok }">
        <Activity :size="18" />
        <span v-if="status?.ok" class="pulse-ring" />
      </div>
      <div>
        <p class="eyebrow">Service health and model history</p>
        <h1>{{ siteName }}</h1>
        <p class="tagline" :class="{ 'tagline--fading': taglineFading }">
          {{ taglines[taglineIndex] }}
        </p>
      </div>
    </div>

    <div class="status-actions">
      <VChip
        class="status-chip"
        :color="status?.ok ? 'success' : 'warning'"
        variant="tonal"
      >
        {{ label }}
      </VChip>
      <span class="status-time">{{ generatedLabel }}</span>
      <div class="status-button-group" aria-label="Dashboard actions">
        <VBtn
          class="status-action-button"
          icon
          size="small"
          variant="tonal"
          :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
          @click="emit('toggle-theme')"
        >
          <component :is="isDark ? Sun : Moon" :size="18" />
          <VTooltip activator="parent" location="bottom">
            {{ isDark ? 'Light mode' : 'Dark mode' }}
          </VTooltip>
        </VBtn>
        <VBtn
          class="status-action-button status-action-button--primary"
          :loading="checking"
          :disabled="checking"
          icon
          size="small"
          variant="flat"
          aria-label="Run checks"
          @click="emit('run-checks')"
        >
          <PlayCircle :size="18" />
          <VTooltip activator="parent" location="bottom">Run checks</VTooltip>
        </VBtn>
        <VBtn
          class="status-action-button"
          :loading="loading"
          :disabled="loading"
          icon
          size="small"
          variant="tonal"
          aria-label="Refresh dashboard"
          @click="emit('refresh')"
        >
          <RefreshCw :size="18" />
          <VTooltip activator="parent" location="bottom">Refresh</VTooltip>
        </VBtn>
      </div>
    </div>
  </header>
</template>
