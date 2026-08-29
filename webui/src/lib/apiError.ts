// Turning a thrown value into text a person can read.
//
// Every /api/v1 failure answers an RFC 9457 ("Problem Details for HTTP
// APIs") body — Content-Type: application/problem+json, shaped
// {"type","title","status","detail","instance","errors"?} — and the backend
// writes user-facing sentences into `detail` for upstream problems (see
// internal/upstream). This module is the single place the UI reads that
// shape, so no view has to re-derive `err?.response?.data?.detail ??
// err.message` — and so a body that is itself JSON is never shown raw, which
// is exactly what used to leak a raw problem document onto the screen.

const GENERIC_MESSAGE = 'Something went wrong. Please try again.'
export const NETWORK_MESSAGE = 'The server could not be reached. Check your connection and try again.'

/** The RFC 9457 problem+json body every /api/v1 handler answers with. */
export interface ApiErrorBody {
    type?: string
    title?: string
    status?: number
    detail?: string
    instance?: string
    errors?: { pointer: string; detail: string }[]
}

function responseData(err: unknown): unknown {
    if (typeof err !== 'object' || err === null) return undefined
    const response = (err as { response?: { data?: unknown } }).response
    return response?.data
}

function responseStatus(err: unknown): number | undefined {
    if (typeof err !== 'object' || err === null) return undefined
    return (err as { response?: { status?: number } }).response?.status
}

// typeSlug mirrors the backend's httperr.Slug: a Problem's `type` is a
// stable URI (e.g. "https://aether.local/probs/upstream_rate_limited") that
// is never fetched — only its last path segment is meant to be read, exactly
// like the old "code" string it replaces.
function typeSlug(type: string): string {
    const i = type.lastIndexOf('/')
    return i >= 0 ? type.slice(i + 1) : type
}

// detailOrTitle reads a Problem's `detail`, falling back to `title`, the
// priority every Problem-derived message follows.
function detailOrTitle(body: ApiErrorBody): string {
    if (typeof body.detail === 'string' && body.detail.trim() !== '') return body.detail.trim()
    if (typeof body.title === 'string' && body.title.trim() !== '') return body.title.trim()
    return ''
}

// unwrapNested defends against an error string that is itself a JSON envelope
// (a double-wrapped body): show the inner sentence, never the document.
function unwrapNested(msg: string): string {
    let current = msg.trim()
    for (let depth = 0; depth < 3 && current.startsWith('{'); depth++) {
        try {
            const parsed = JSON.parse(current) as ApiErrorBody
            const inner = detailOrTitle(parsed)
            if (inner === '') return ''
            current = inner
        } catch {
            return '' // not JSON after all, but starts with '{' — not showable
        }
    }
    return current.startsWith('{') ? '' : current
}

/**
 * apiErrorMessage extracts a human-readable message from anything thrown by the
 * API layer, falling back to `fallback` when the error carries nothing useful.
 */
export function apiErrorMessage(err: unknown, fallback: string = GENERIC_MESSAGE): string {
    const data = responseData(err)

    if (typeof data === 'string' && data.trim() !== '') {
        const unwrapped = unwrapNested(data)
        if (unwrapped !== '') return unwrapped
    }

    if (typeof data === 'object' && data !== null) {
        const candidate = detailOrTitle(data as ApiErrorBody)
        if (candidate !== '') {
            const unwrapped = unwrapNested(candidate)
            if (unwrapped !== '') return unwrapped
        }
    }

    if (typeof err === 'object' && err !== null) {
        const e = err as { message?: unknown; request?: unknown }
        // A request that got no response at all is a connectivity problem, and
        // axios's own "Network Error" doesn't say that in user terms.
        if (responseStatus(err) === undefined && e.request !== undefined) {
            return NETWORK_MESSAGE
        }
        if (typeof e.message === 'string' && e.message.trim() !== '') {
            return e.message
        }
    }

    return fallback
}

/**
 * isCanceledError reports whether the request was aborted deliberately (an
 * AbortController the UI triggered), as opposed to having failed. Callers use it
 * to stay silent: a cancel the user asked for is not an error to report, and
 * axios surfaces it as a rejection like any other.
 */
export function isCanceledError(err: unknown): boolean {
    if (typeof err !== 'object' || err === null) return false
    const e = err as { code?: unknown; name?: unknown; message?: unknown }
    // axios sets code ERR_CANCELED; a raw AbortError (fetch, or an abort that
    // fires before axios wraps it) only carries the name.
    return e.code === 'ERR_CANCELED' || e.name === 'CanceledError' || e.name === 'AbortError'
}

/**
 * isRateLimitError reports whether the failure was a rate limit — ours (429) or
 * an upstream provider's — so the UI can invite a retry rather than report a
 * hard failure.
 */
export function isRateLimitError(err: unknown): boolean {
    if (responseStatus(err) === 429) return true
    const data = responseData(err)
    if (typeof data === 'object' && data !== null) {
        const type = (data as ApiErrorBody).type
        return typeof type === 'string' && typeSlug(type) === 'upstream_rate_limited'
    }
    return false
}
