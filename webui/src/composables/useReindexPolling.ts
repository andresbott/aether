import { listExecutions } from '@/lib/api/Tasks'

const TERMINAL = new Set(['complete', 'failed', 'panicked', 'canceled', 'cancel_error', 'unknown'])

export interface PollReindexOptions {
    intervalMs?: number
    timeoutMs?: number
    signal?: AbortSignal
}

// ReindexPollResult reports how a batch of re-index jobs settled:
//   failed  - jobs observed reaching a non-`complete` terminal status.
//   pending - jobs never confirmed complete before polling ended: the timeout
//             elapsed, the poll was aborted, the executions fetch kept failing,
//             or the job never surfaced in the runner's (bounded) history.
// Either being non-zero means the index is NOT known to reflect the
// write, so a caller must not report an unqualified success.
export interface ReindexPollResult {
    failed: number
    pending: number
}

function wait(ms: number, signal?: AbortSignal): Promise<void> {
    return new Promise((resolve) => {
        const t = setTimeout(resolve, ms)
        signal?.addEventListener(
            'abort',
            () => {
                clearTimeout(t)
                resolve()
            },
            { once: true }
        )
    })
}

// pollReindex polls the executions list until every id in `ids` reaches a
// terminal status, reporting how the batch settled. An id seen running that then
// drops out of the runner's bounded history is treated as complete (it landed
// and rolled out); an id that has NOT yet been seen is kept pending — never
// assumed complete merely because it is absent, which would defeat the point of
// awaiting the re-index. Aborting the signal, hitting the timeout, or a
// persistent poll error all leave the still-unconfirmed ids pending rather than
// reporting them complete. Resolves immediately for an empty list; never rejects.
export async function pollReindex(
    ids: string[],
    opts: PollReindexOptions = {}
): Promise<ReindexPollResult> {
    const pending = new Set(ids)
    if (pending.size === 0) return { failed: 0, pending: 0 }
    const interval = opts.intervalMs ?? 500
    const timeout = opts.timeoutMs ?? 30_000
    const { signal } = opts
    const deadline = Date.now() + timeout
    const seen = new Set<string>()
    let failed = 0

    while (pending.size > 0 && Date.now() < deadline && !signal?.aborted) {
        let byId: Map<string, string> | null = null
        try {
            const list = await listExecutions(signal)
            byId = new Map(list.map((e) => [e.id, e.status]))
        } catch {
            // A failed round tells us nothing about the jobs, so it must conclude
            // nothing — treating absence as completion on an outage would report
            // false success.
            byId = null
        }
        if (byId) {
            for (const id of [...pending]) {
                const status = byId.get(id)
                if (status === undefined) {
                    // Absent: complete only if it was already seen running (it
                    // finished and rolled out of history). An id never seen has
                    // not surfaced yet — keep waiting for it.
                    if (seen.has(id)) pending.delete(id)
                    continue
                }
                seen.add(id)
                if (TERMINAL.has(status)) {
                    if (status !== 'complete') failed++
                    pending.delete(id)
                }
            }
        }
        if (pending.size > 0) await wait(interval, signal)
    }
    return { failed, pending: pending.size }
}
