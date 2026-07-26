import { describe, it, expect } from 'vitest'
import { apiErrorMessage, isRateLimitError } from '@/lib/apiError'

// The server answers /api/v1 failures as {"error": "<sentence>", "code": "<slug>"}.
// apiErrorMessage is the single place the UI turns any thrown value into text a
// person can read — no raw JSON, no "[object Object]", never empty.
describe('apiErrorMessage', () => {
    it('uses the server sentence from an axios-style error', () => {
        const err = {
            response: {
                status: 502,
                data: {
                    error: 'Cover Art Archive is temporarily unavailable. Try again in a few minutes.',
                    code: 'upstream_error'
                }
            }
        }
        expect(apiErrorMessage(err)).toBe(
            'Cover Art Archive is temporarily unavailable. Try again in a few minutes.'
        )
    })

    // Regression: a double-wrapped body used to put a raw JSON document on
    // screen. Even if one ever reaches the client, show the inner sentence.
    it('unwraps a JSON document that arrived in the error field', () => {
        const err = {
            response: {
                status: 502,
                data: {
                    error: '{"error":"cover art archive lookup failed: status 500","code":"upstream_error"}',
                    code: 502
                }
            }
        }
        expect(apiErrorMessage(err)).toBe('cover art archive lookup failed: status 500')
    })

    it('never returns a string that looks like JSON', () => {
        const err = { response: { status: 500, data: { error: '{"nested":{"deep":true}}' } } }
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
})

// Rate limiting is worth telling apart: the UI can invite a retry instead of
// reporting a hard failure.
describe('isRateLimitError', () => {
    it('detects a 429 response', () => {
        expect(isRateLimitError({ response: { status: 429, data: {} } })).toBe(true)
    })

    it('detects the upstream_rate_limited code', () => {
        expect(
            isRateLimitError({ response: { status: 502, data: { code: 'upstream_rate_limited' } } })
        ).toBe(true)
    })

    it('is false for other failures', () => {
        expect(isRateLimitError({ response: { status: 502, data: { code: 'upstream_error' } } })).toBe(
            false
        )
        expect(isRateLimitError(new Error('boom'))).toBe(false)
    })
})
