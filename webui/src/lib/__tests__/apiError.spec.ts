import { describe, it, expect } from 'vitest'
import { apiErrorMessage, isCanceledError, isRateLimitError } from '@/lib/apiError'

// The server answers /api/v1 failures as an RFC 9457 problem+json body:
// {"type","title","status","detail","instance","errors"?}. apiErrorMessage is
// the single place the UI turns any thrown value into text a person can
// read — no raw JSON, no "[object Object]", never empty.
describe('apiErrorMessage', () => {
    it('uses the server sentence from a Problem body', () => {
        const err = {
            response: {
                status: 502,
                data: {
                    type: 'https://aether.local/probs/upstream_error',
                    title: 'Upstream error',
                    status: 502,
                    detail: 'Cover Art Archive is temporarily unavailable. Try again in a few minutes.',
                    instance: '/api/v1/metadata/pictures/candidates'
                }
            }
        }
        expect(apiErrorMessage(err)).toBe(
            'Cover Art Archive is temporarily unavailable. Try again in a few minutes.'
        )
    })

    it('falls back to the title when detail is absent', () => {
        const err = {
            response: {
                status: 404,
                data: {
                    type: 'https://aether.local/probs/not_found',
                    title: 'Not found',
                    status: 404,
                    instance: '/api/v1/libraries/9'
                }
            }
        }
        expect(apiErrorMessage(err)).toBe('Not found')
    })

    // Regression: a double-wrapped body used to put a raw JSON document on
    // screen. Even if one ever reaches the client, show the inner sentence.
    it('unwraps a JSON document that arrived in the detail field', () => {
        const err = {
            response: {
                status: 502,
                data: {
                    type: 'https://aether.local/probs/upstream_error',
                    title: 'Upstream error',
                    status: 502,
                    detail:
                        '{"type":"https://aether.local/probs/upstream_error","title":"Upstream error","status":500,"detail":"cover art archive lookup failed: status 500"}'
                }
            }
        }
        expect(apiErrorMessage(err)).toBe('cover art archive lookup failed: status 500')
    })

    it('never returns a string that looks like JSON', () => {
        const err = {
            response: { status: 500, data: { detail: '{"nested":{"deep":true}}' } }
        }
        const msg = apiErrorMessage(err)
        expect(msg.startsWith('{')).toBe(false)
    })

    it('falls back to the message of a plain Error', () => {
        expect(apiErrorMessage(new Error('Network Error'))).toBe('Network Error')
    })

    it('uses the caller fallback when there is nothing to show', () => {
        expect(apiErrorMessage({}, 'Could not load covers.')).toBe('Could not load covers.')
        expect(apiErrorMessage(null, 'Could not load covers.')).toBe('Could not load covers.')
    })

    it('has a generic default when the caller gives no fallback', () => {
        expect(apiErrorMessage(undefined)).toBe('Something went wrong. Please try again.')
    })

    it('describes a plain string body', () => {
        const err = { response: { status: 400, data: 'wrong api call' } }
        expect(apiErrorMessage(err)).toBe('wrong api call')
    })

    it('explains a network failure with no response', () => {
        const err = { message: 'Network Error', request: {}, code: 'ERR_NETWORK' }
        expect(apiErrorMessage(err)).toContain('server could not be reached')
    })

    it('returns NETWORK_MESSAGE for errors with request but no response', () => {
        // This is the shape axios throws when the backend is unreachable.
        const err = { message: 'Network Error', request: {}, code: 'ERR_NETWORK' }
        expect(apiErrorMessage(err)).toBe(
            'The server could not be reached. Check your connection and try again.'
        )
    })

    it('distinguishes server errors from network failures', () => {
        // A 500 with a response is NOT a network failure — the server answered.
        const err = {
            response: { status: 500, data: { detail: 'Internal server error' } },
            request: {}
        }
        expect(apiErrorMessage(err)).toBe('Internal server error')
        expect(apiErrorMessage(err)).not.toContain('server could not be reached')
    })

    it('does not misclassify an AbortError as a network outage', () => {
        // User-canceled requests are NOT network failures.
        const abortErr = { name: 'AbortError', message: 'The operation was aborted' }
        expect(apiErrorMessage(abortErr)).toBe('The operation was aborted')
        expect(apiErrorMessage(abortErr)).not.toContain('server could not be reached')
    })

    it('does not misclassify a CanceledError as a network outage', () => {
        const cancelErr = { code: 'ERR_CANCELED', message: 'canceled', name: 'CanceledError' }
        expect(apiErrorMessage(cancelErr)).toBe('canceled')
        expect(apiErrorMessage(cancelErr)).not.toContain('server could not be reached')
    })
})

// Rate limiting is worth telling apart: the UI can invite a retry instead of
// reporting a hard failure.
describe('isRateLimitError', () => {
    it('detects a 429 response', () => {
        expect(isRateLimitError({ response: { status: 429, data: {} } })).toBe(true)
    })

    it('detects the upstream_rate_limited type slug', () => {
        expect(
            isRateLimitError({
                response: {
                    status: 502,
                    data: { type: 'https://aether.local/probs/upstream_rate_limited' }
                }
            })
        ).toBe(true)
    })

    it('is false for other failures', () => {
        expect(
            isRateLimitError({
                response: { status: 502, data: { type: 'https://aether.local/probs/upstream_error' } }
            })
        ).toBe(false)
        expect(isRateLimitError(new Error('boom'))).toBe(false)
    })
})

describe('isCanceledError', () => {
    it('recognises a deliberate abort in each shape it arrives as', () => {
        // axios wraps an aborted request as a CanceledError with this code.
        expect(isCanceledError({ code: 'ERR_CANCELED', message: 'canceled' })).toBe(true)
        expect(isCanceledError({ name: 'CanceledError' })).toBe(true)
        // A raw AbortError, when the abort fires before axios wraps it.
        expect(isCanceledError({ name: 'AbortError' })).toBe(true)
    })

    it('does not mistake a real failure for a cancel', () => {
        expect(isCanceledError(new Error('boom'))).toBe(false)
        expect(isCanceledError({ response: { status: 502, data: {} } })).toBe(false)
        expect(isCanceledError({ code: 'ERR_NETWORK' })).toBe(false)
        expect(isCanceledError(null)).toBe(false)
        expect(isCanceledError(undefined)).toBe(false)
        expect(isCanceledError('canceled')).toBe(false)
    })
})
