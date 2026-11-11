// Transient-failure retry for the SPA's HTTP calls. A phone that wakes, switches
// from Wi-Fi to mobile data or simply drops a packet fails a request that would
// succeed a second later; retrying it here keeps that blip from surfacing as an
// error, or worse, from leaving a view broken until a reload. Only failures
// that say nothing about the request itself are retried: no answer at all, or a
// gateway that could not reach the server. Anything the server answered — a
// 4xx, a 500 — is final.

/** HTTP statuses a proxy or load balancer answers while the server is unreachable. */
const TRANSIENT_STATUSES = new Set([502, 503, 504])

/**
 * A response that arrived with one of the transient gateway statuses, thrown so
 * the retry loop and {@link isNetworkFailure} can tell it from a real answer.
 */
export class TransientHttpError extends Error {
    constructor(readonly status: number) {
        super(`HTTP error! status: ${status}`)
        this.name = 'TransientHttpError'
    }
}

export function isTransientStatus(status: number): boolean {
    return TRANSIENT_STATUSES.has(status)
}

/**
 * True when `err` means the server was never reached: the desktop transport's
 * bare-string rejection, fetch's TypeError
 * ("Failed to fetch", "Load failed", "NetworkError when attempting…"), axios'
 * ERR_NETWORK / timeout codes, or a transient gateway status. An abort is not
 * one — whoever aborted wanted the request gone.
 */
export function isNetworkFailure(err: unknown): boolean {
    if (err instanceof TransientHttpError) return true
    // The desktop app's HTTP plugin rejects a request that got no answer with a
    // bare string (lib/apiError.ts reads it the same way).
    if (typeof err === 'string') return true
    if (typeof err !== 'object' || err === null) return false
    const e = err as { name?: string; code?: string; response?: unknown; isAxiosError?: boolean }
    if (e.name === 'AbortError' || e.name === 'CanceledError' || e.code === 'ERR_CANCELED') {
        return false
    }
    if (e.isAxiosError) {
        if (e.response) return isTransientStatus((e.response as { status: number }).status)
        return e.code === 'ERR_NETWORK' || e.code === 'ECONNABORTED' || e.code === 'ETIMEDOUT'
    }
    return err instanceof TypeError
}

// The waits before the second and third attempts. Short: these cover a blip,
// not an outage — vue-query's own retries and the connectivity probe
// (useConnectivity) take over for anything longer. Specs run without the waits
// (many reload their modules, so a setup-file override would not stick); one
// about the timing sets its own with setRetryDelaysForTests.
let retryDelaysMs: readonly number[] = import.meta.env.MODE === 'test' ? [0, 0] : [400, 1500]

/** The waits between attempts; its length is the number of retries. */
export function retryDelays(): readonly number[] {
    return retryDelaysMs
}

/** Test hook: specs run with zero waits so a retried failure costs no time. */
export function setRetryDelaysForTests(delays: readonly number[]): void {
    retryDelaysMs = delays
}

const sleep = (ms: number): Promise<void> =>
    ms > 0 ? new Promise((resolve) => setTimeout(resolve, ms)) : Promise.resolve()

/**
 * Runs `attempt`, and again after each of {@link retryDelays} while it fails
 * with an error `shouldRetry` accepts (by default {@link isNetworkFailure}).
 * The last failure is rethrown unchanged. `signal`, when given, stops the
 * retries once it is aborted.
 */
export async function withRetry<T>(
    attempt: () => Promise<T>,
    opts: { shouldRetry?: (err: unknown) => boolean; signal?: AbortSignal | null } = {}
): Promise<T> {
    const shouldRetry = opts.shouldRetry ?? isNetworkFailure
    const delays = retryDelaysMs
    for (let i = 0; ; i++) {
        try {
            return await attempt()
        } catch (err) {
            if (i >= delays.length || opts.signal?.aborted || !shouldRetry(err)) throw err
            await sleep(delays[i] ?? 0)
            if (opts.signal?.aborted) throw err
        }
    }
}
