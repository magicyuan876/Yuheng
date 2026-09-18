// Prometheus-style counters and one latency histogram, rendered as text.

export class Metrics {
  readonly counters = new Map<string, number>()
  private readonly buckets = [0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
  private storeCounts = new Array<number>(this.buckets.length + 1).fill(0)
  private storeSum = 0
  private storeTotal = 0

  inc(name: string, by = 1): void {
    this.counters.set(name, (this.counters.get(name) ?? 0) + by)
  }

  get(name: string): number {
    return this.counters.get(name) ?? 0
  }

  observeStoreSeconds(seconds: number): void {
    this.storeTotal += 1
    this.storeSum += seconds
    let i = 0
    while (i < this.buckets.length && seconds > this.buckets[i]) i++
    this.storeCounts[i] += 1
  }

  /** Renders every metric in the Prometheus exposition format. */
  render(gauges: Record<string, number>): string {
    const lines: string[] = []
    for (const [name, value] of [...this.counters.entries()].sort()) {
      lines.push(`# TYPE ${name} counter`, `${name} ${value}`)
    }
    for (const [name, value] of Object.entries(gauges).sort()) {
      lines.push(`# TYPE ${name} gauge`, `${name} ${value}`)
    }
    lines.push('# TYPE collab_store_duration_seconds histogram')
    let cumulative = 0
    for (let i = 0; i < this.buckets.length; i++) {
      cumulative += this.storeCounts[i]
      lines.push(`collab_store_duration_seconds_bucket{le="${this.buckets[i]}"} ${cumulative}`)
    }
    lines.push(`collab_store_duration_seconds_bucket{le="+Inf"} ${this.storeTotal}`)
    lines.push(`collab_store_duration_seconds_sum ${this.storeSum}`)
    lines.push(`collab_store_duration_seconds_count ${this.storeTotal}`)
    return `${lines.join('\n')}\n`
  }
}

/** Counter names, in one place so dashboards and tests agree. */
export const M = {
  connectionsAccepted: 'collab_connections_accepted_total',
  authRejected: 'collab_auth_rejected_total',
  authBackendUnavailable: 'collab_auth_backend_unavailable_total',
  storeOk: 'collab_store_success_total',
  storeConflict: 'collab_store_conflict_total',
  storeFailed: 'collab_store_failed_total',
  storeRejected: 'collab_store_rejected_total',
  storeTooLarge: 'collab_store_too_large_total',
  readonlyDropped: 'collab_readonly_updates_dropped_total',
  rateLimited: 'collab_connections_rate_limited_total',
  recheckDowngraded: 'collab_recheck_downgraded_total',
  recheckClosed: 'collab_recheck_closed_total',
  replaceApplied: 'collab_replace_applied_total',
  evicted: 'collab_evictions_total',
  redisRelayed: 'collab_redis_messages_relayed_total',
} as const
