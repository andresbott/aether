import { describe, it, expect, vi, afterEach } from 'vitest'
import { AxiosError } from 'axios'
import {
    isNetworkFailure,
    retryDelays,
    setRetryDelaysForTests,
    TransientHttpError,
    withRetry
} from '@/lib/retry'

const defaultDelays = retryDelays()

afterEach(() => {
    setRetryDelaysForTests(defaultDelays)
    vi.useRealTimers()
})

describe('isNetworkFailure', () => {
    it('recognises a request that got no answer', () => {
        expect(isNetworkFailure(new TypeError('Failed to fetch'))).toBe(true)
        // The desktop transport rejects with a bare string.
        expect(isNetworkFailure('error sending request for url (…)')).toBe(true)
        expect(isNetworkFailure(new AxiosError('Network Error', 'ERR_NETWORK'))).toBe(true)
        expect(isNetworkFailure(new AxiosError('timeout', 'ECONNABORTED'))).toBe(true)
    })

    it('counts a gateway that could not reach the server', () => {
        expect(isNetworkFailure(new TransientHttpError(503))).toBe(true)
        const gateway = new AxiosError('bad gateway', 'ERR_BAD_RESPONSE', undefined, undefined, {
            status: 502
        } as never)
        expect(isNetworkFailure(gateway)).toBe(true)
    })

    it('does not count anything the server answered, nor an abort', () => {
        const refused = new AxiosError('nope', 'ERR_BAD_REQUEST', undefined, undefined, {
            status: 404
        } as never)
        expect(isNetworkFailure(refused)).toBe(false)
        const crashed = new AxiosError('boom', 'ERR_BAD_RESPONSE', undefined, undefined, {
            status: 500
        } as never)
        expect(isNetworkFailure(crashed)).toBe(false)
        expect(isNetworkFailure(new DOMException('aborted', 'AbortError'))).toBe(false)
        expect(isNetworkFailure(new AxiosError('canceled', 'ERR_CANCELED'))).toBe(false)
        expect(isNetworkFailure(new Error('Subsonic said no'))).toBe(false)
        expect(isNetworkFailure(null)).toBe(false)
    })
})

describe('withRetry', () => {
    it('retries a network failure and returns the first success', async () => {
        const attempt = vi
            .fn()
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce('ok')
        await expect(withRetry(attempt)).resolves.toBe('ok')
        expect(attempt).toHaveBeenCalledTimes(2)
    })

    it('gives up after its retries and rethrows the last failure', async () => {
        const failure = new TypeError('Failed to fetch')
        const attempt = vi.fn().mockRejectedValue(failure)
        await expect(withRetry(attempt)).rejects.toBe(failure)
        expect(attempt).toHaveBeenCalledTimes(1 + retryDelays().length)
    })

    it('does not retry an answer', async () => {
        const attempt = vi.fn().mockRejectedValue(new Error('HTTP error! status: 500'))
        await expect(withRetry(attempt)).rejects.toThrow('500')
        expect(attempt).toHaveBeenCalledTimes(1)
    })

    it('stops once its signal is aborted', async () => {
        const controller = new AbortController()
        const attempt = vi.fn().mockImplementation(() => {
            controller.abort()
            return Promise.reject(new TypeError('Failed to fetch'))
        })
        await expect(withRetry(attempt, { signal: controller.signal })).rejects.toThrow()
        expect(attempt).toHaveBeenCalledTimes(1)
    })

    it('waits the configured delays between attempts', async () => {
        vi.useFakeTimers()
        setRetryDelaysForTests([400, 1500])
        const attempt = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
        const result = withRetry(attempt).catch(() => 'failed')

        await vi.advanceTimersByTimeAsync(0)
        expect(attempt).toHaveBeenCalledTimes(1)
        await vi.advanceTimersByTimeAsync(400)
        expect(attempt).toHaveBeenCalledTimes(2)
        await vi.advanceTimersByTimeAsync(1499)
        expect(attempt).toHaveBeenCalledTimes(2)
        await vi.advanceTimersByTimeAsync(1)
        expect(attempt).toHaveBeenCalledTimes(3)
        await expect(result).resolves.toBe('failed')
    })
})
