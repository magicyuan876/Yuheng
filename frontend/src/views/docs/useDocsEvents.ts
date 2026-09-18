import { fetchEventSource } from '@microsoft/fetch-event-source'
import { onBeforeUnmount, watch, type Ref } from 'vue'

import { getApiBaseUrl } from '@/utils/api-base'

/** One docs domain event as the SSE stream delivers it (internal/docs/events). */
export interface DocsEvent {
  type: string
  tenant_id?: number
  space_id?: string
  page_id?: string
  actor_id?: string
  at?: string
  payload?: Record<string, unknown>
}

/**
 * Subscribes to GET /api/v1/docs/events narrowed to one space and hands every
 * event to `onEvent`. The stream is a refresh hint: callers re-fetch what
 * changed rather than trusting payloads as truth. Reconnects with backoff and
 * follows the space ref.
 */
export function useDocsEvents(spaceId: Ref<string | undefined>, onEvent: (ev: DocsEvent) => void): void {
  let controller: AbortController | null = null
  let retryTimer: number | null = null
  let attempts = 0

  const stop = () => {
    if (retryTimer !== null) {
      window.clearTimeout(retryTimer)
      retryTimer = null
    }
    controller?.abort()
    controller = null
  }

  const start = (sid: string) => {
    stop()
    const token = localStorage.getItem('yuheng_token')
    if (!token) return
    const headers: Record<string, string> = { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' }
    const tenant = localStorage.getItem('yuheng_selected_tenant_id')
    if (tenant) headers['X-Tenant-ID'] = tenant
    const ctl = new AbortController()
    controller = ctl
    const url = `${getApiBaseUrl()}/api/v1/docs/events?space=${encodeURIComponent(sid)}`

    fetchEventSource(url, {
      method: 'GET',
      headers,
      signal: ctl.signal,
      openWhenHidden: true,
      onopen: async (res) => {
        if (!res.ok) throw new Error(`docs events: HTTP ${res.status}`)
        attempts = 0
      },
      onmessage: (msg) => {
        if (!msg.event || msg.event === 'ready' || !msg.data) return
        try {
          const parsed = JSON.parse(msg.data) as DocsEvent
          onEvent({ ...parsed, type: parsed.type || msg.event })
        } catch {
          // A malformed frame is dropped; the next refresh resyncs.
        }
      },
      onerror: () => {
        // Returning a number would let the library retry immediately; we
        // schedule our own backoff instead so a dead server is not hammered.
        throw new Error('docs events: reconnect')
      },
    }).catch(() => {
      if (ctl.signal.aborted) return
      attempts += 1
      const delay = Math.min(30_000, 1000 * 2 ** Math.min(attempts, 5))
      retryTimer = window.setTimeout(() => {
        if (spaceId.value === sid) start(sid)
      }, delay)
    })
  }

  watch(spaceId, (sid) => {
    attempts = 0
    if (sid) start(sid)
    else stop()
  }, { immediate: true })

  onBeforeUnmount(stop)
}
