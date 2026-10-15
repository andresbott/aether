import { describe, it, expect, vi, afterEach } from 'vitest'

// The installed transport is module state, so each test starts from a fresh copy.
async function loadTransport() {
    vi.resetModules()
    return import('@/lib/host/transport')
}

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('httpFetch', () => {
    // Resolved on every call, not captured at import: a stub installed after the
    // module loaded must still answer.
    it('uses the global fetch of the moment when no transport is installed', async () => {
        const { httpFetch } = await loadTransport()
        const response = new Response('ok')
        const stub = vi.fn().mockResolvedValue(response)
        vi.stubGlobal('fetch', stub)
        const init = { method: 'POST', body: 'x' }

        await expect(httpFetch('http://music.lan:8075/rest/ping.view', init)).resolves.toBe(response)
        expect(stub).toHaveBeenCalledWith('http://music.lan:8075/rest/ping.view', init)
    })

    it('routes every later call through an installed transport', async () => {
        const { httpFetch, installTransport } = await loadTransport()
        const pageFetch = vi.fn()
        vi.stubGlobal('fetch', pageFetch)
        const response = new Response('from rust')
        const transport = vi.fn().mockResolvedValue(response)

        installTransport(transport)

        await expect(httpFetch('http://music.lan:8075/api/v0/me')).resolves.toBe(response)
        expect(transport).toHaveBeenCalledWith('http://music.lan:8075/api/v0/me', undefined)
        expect(pageFetch).not.toHaveBeenCalled()
    })
})
