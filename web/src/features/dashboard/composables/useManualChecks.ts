import { onUnmounted, shallowRef } from 'vue'
import type { ManualCheckJobsResponse, ManualCheckRunResponse } from '@/types'
import { parseModelIdentity } from '@/features/dashboard/utils/modelInventory'

interface UseManualChecksOptions {
  onComplete?: () => void | Promise<void>
  pollMs?: number
}

interface PendingGroup {
  ids: string[]
  modelId?: string
}

const TERMINAL_STATES = new Set(['completed', 'archived', 'expired', 'not_found'])

/** Starts manual checks and polls queue state until matching jobs finish. */
export function useManualChecks(options: UseManualChecksOptions = {}) {
  const pollMs = options.pollMs ?? 1_500
  const globalChecking = shallowRef(false)
  const checkingModelIds = shallowRef<string[]>([])
  const error = shallowRef<string | null>(null)
  const pendingGroups = new Map<string, PendingGroup>()
  let timer: number | undefined

  async function runAllChecks() {
    error.value = null
    globalChecking.value = true
    try {
      const response = await requestRun({ scope: 'all' })
      trackGroup(`all:${Date.now()}`, { ids: response.jobs.map(job => job.id) })
    } catch (err) {
      globalChecking.value = false
      error.value = messageForError(err, 'Unable to start checks')
    }
  }

  async function runModelCheck(identity: string) {
    const parsed = parseModelIdentity(identity)
    if (!parsed) {
      error.value = 'Invalid model selection'
      return
    }
    error.value = null
    setModelChecking(identity, true)
    try {
      const response = await requestRun({ scope: 'model', provider_id: parsed.providerId, model_id: modelIdFromIdentity(identity) })
      trackGroup(`model:${identity}:${Date.now()}`, {
        ids: response.jobs.map(job => job.id),
        modelId: identity
      })
    } catch (err) {
      setModelChecking(identity, false)
      error.value = messageForError(err, 'Unable to start model check')
    }
  }

  async function requestRun(body: { scope: 'all' | 'model', provider_id?: string, model_id?: string }) {
    const response = await fetch('/api/checks/runs', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(body)
    })
    if (!response.ok) {
      throw new Error(`Manual check API returned ${response.status}`)
    }
    return await response.json() as ManualCheckRunResponse
  }

  function trackGroup(key: string, group: PendingGroup) {
    if (group.ids.length === 0) {
      finishGroup(group)
      return
    }
    pendingGroups.set(key, group)
    schedulePoll()
  }

  function schedulePoll() {
    if (timer !== undefined) return
    timer = window.setTimeout(() => {
      timer = undefined
      void poll()
    }, pollMs)
  }

  async function poll() {
    const ids = [...new Set([...pendingGroups.values()].flatMap(group => group.ids))]
    if (ids.length === 0) return
    try {
      const params = new URLSearchParams()
      params.set('ids', ids.join(','))
      const response = await fetch(`/api/checks/jobs?${params.toString()}`, {
        headers: { Accept: 'application/json' }
      })
      if (!response.ok) {
        throw new Error(`Manual check status API returned ${response.status}`)
      }
      const payload = await response.json() as ManualCheckJobsResponse
      const states = new Map(payload.jobs.map(job => [job.id, job]))
      const completedGroups: PendingGroup[] = []
      for (const [key, group] of pendingGroups) {
        const done = group.ids.every(id => TERMINAL_STATES.has(states.get(id)?.state ?? 'not_found'))
        if (!done) continue
        pendingGroups.delete(key)
        completedGroups.push(group)
        for (const id of group.ids) {
          const job = states.get(id)
          if (job?.state === 'archived' || job?.error) {
            error.value = job.error || 'A manual check failed'
          }
        }
      }
      if (completedGroups.length > 0) {
        try {
          await options.onComplete?.()
        } finally {
          for (const group of completedGroups) {
            finishGroup(group)
          }
        }
      }
    } catch (err) {
      error.value = messageForError(err, 'Unable to poll manual checks')
    } finally {
      if (pendingGroups.size > 0) {
        schedulePoll()
      }
    }
  }

  function finishGroup(group: PendingGroup) {
    if (group.modelId) {
      setModelChecking(group.modelId, false)
      return
    }
    globalChecking.value = false
  }

  function setModelChecking(modelId: string, value: boolean) {
    const next = new Set(checkingModelIds.value)
    if (value) {
      next.add(modelId)
    } else {
      next.delete(modelId)
    }
    checkingModelIds.value = [...next]
  }

  onUnmounted(() => {
    if (timer !== undefined) window.clearTimeout(timer)
  })

  return {
    globalChecking,
    checkingModelIds,
    error,
    runAllChecks,
    runModelCheck
  }
}

function messageForError(err: unknown, fallback: string) {
  return err instanceof Error ? err.message : fallback
}

function modelIdFromIdentity(identity: string) {
  const slash = identity.indexOf('/')
  if (slash < 0) return identity
  const raw = identity.slice(slash + 1)
  const base64 = raw.replace(/-/g, '+').replace(/_/g, '/')
  const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, '=')
  return atob(padded)
}
