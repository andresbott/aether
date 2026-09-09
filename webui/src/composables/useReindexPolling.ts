import { listExecutions } from '@/lib/api/Tasks'

const TERMINAL = new Set(['complete', 'failed', 'panicked', 'canceled', 'cancel_error', 'unknown'])

export interface PollReindexOptions {
    intervalMs?: number
    timeoutMs?: number
}

// pollReindex polls the executions list until every id in `ids` has reached a
// terminal status, and reports how many did NOT end in `complete`. Ids that
// have rolled out of the runner's bounded history are treated as complete (the
// write already landed; the job almost certainly finished). Resolves
// immediately for an empty list. Never rejects — a poll error ends that round
// and the timeout eventually resolves with what is known.
export async function pollReindex(
    ids: string[],
    opts: PollReindexOptions = {}
): Promise<{ failed: number }> {
    const pending = new Set(ids)
    if (pending.size === 0) return { failed: 0 }
    const interval = opts.intervalMs ?? 500
    const timeout = opts.timeoutMs ?? 30_000
    const deadline = Date.now() + timeout
    let failed = 0

    while (pending.size > 0 && Date.now() < deadline) {
        let byId: Map<string, string>
        try {
            const list = await listExecutions()
            byId = new Map(list.map((e) => [e.id, e.status]))
        } catch {
            byId = new Map()
        }
        for (const id of [...pending]) {
            const status = byId.get(id)
            if (status === undefined) {
                // Not in the current window: if we have polled at least once and
                // it is gone, assume it completed and rolled out of history.
                if (byId.size > 0) pending.delete(id)
                continue
            }
            if (TERMINAL.has(status)) {
                if (status !== 'complete') failed++
                pending.delete(id)
            }
        }
        if (pending.size > 0) await new Promise((r) => setTimeout(r, interval))
    }
    return { failed }
}
