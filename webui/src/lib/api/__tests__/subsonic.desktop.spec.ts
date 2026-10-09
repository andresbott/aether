import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// The subsonic client inside the desktop app: the server is the user's setting
// and every request rides the host transport (tauri-plugin-http in the app).
//
// The transport is a plain function, not a vi.fn: a spy settles every promise it
// returns, which would hide an unhandled rejection from the test.
const transport = vi.hoisted(() => ({
    sent: [] as string[],
    answer: (_url: string): Promise<Response> => Promise.reject(new Error('no answer set'))
}))

vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => true,
        serverUrl: ref<string | null>('http://music.lan:8075'),
        httpFetch: (input: RequestInfo | URL) => {
            transport.sent.push(String(input))
            return transport.answer(String(input))
        }
    }
})

import { serverUrl } from '@/lib/host'
import { subsonicClient } from '@/lib/api/subsonic'

const SERVER = 'http://music.lan:8075'

const okEnvelope = async () =>
    new Response(JSON.stringify({ 'subsonic-response': { status: 'ok' } }), {
        headers: { 'Content-Type': 'application/json' }
    })

// What the plugin rejects with when a request fails: its text names the request
// URL, apiKey included.
const pluginFailure = async (url: string): Promise<Response> => {
    throw `error sending request for url (${url})`
}

function sentUrls(): URL[] {
    return transport.sent.map((url) => new URL(url))
}

beforeEach(() => {
    serverUrl.value = SERVER
    transport.sent = []
    transport.answer = okEnvelope
    subsonicClient.setApiKey('aether_key')
})

afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    subsonicClient.clearApiKey()
})

describe('subsonic client in the desktop app', () => {
    it('sends /rest calls to the configured server through the host transport', async () => {
        await expect(subsonicClient.ping()).resolves.toBe(true)

        const [url] = sentUrls()
        expect(url.origin).toBe(SERVER)
        expect(url.pathname).toBe('/rest/ping.view')
        expect(url.searchParams.get('apiKey')).toBe('aether_key')
    })

    // Media URLs are handed to <audio>/<img> as they are, so they must name the
    // server too, not the window's tauri:// origin.
    it('builds media URLs on the server it was configured with', () => {
        expect(new URL(subsonicClient.getStreamUrl('tr-1')).origin).toBe(SERVER)

        serverUrl.value = 'https://aether.example.org'
        subsonicClient.initWithDefaults()

        expect(new URL(subsonicClient.getCoverArtUrl('al-1', 200)).origin).toBe(
            'https://aether.example.org'
        )
    })

    // The window's CSP confines connect-src to IPC, so a beacon could not leave
    // the app; the save goes through the transport instead.
    it('hands the final queue save to the host transport, not a beacon', () => {
        const beacon = vi.fn(() => true)
        vi.stubGlobal('navigator', { sendBeacon: beacon })

        expect(subsonicClient.savePlayQueueBeacon(['tr-1', 'tr-2'], 1, 105000)).toBe(true)

        expect(beacon).not.toHaveBeenCalled()
        const [url] = sentUrls()
        expect(url.origin).toBe(SERVER)
        expect(url.pathname).toBe('/rest/savePlayQueueByIndex.view')
        expect(url.searchParams.getAll('id')).toEqual(['tr-1', 'tr-2'])
        expect(url.searchParams.get('currentIndex')).toBe('1')
        expect(url.searchParams.get('position')).toBe('105000')
    })

    // Fire-and-forget during a reload or quit: a failed save must not surface
    // as an unhandled rejection.
    it('swallows a failed final queue save', async () => {
        transport.answer = pluginFailure
        const unhandled = vi.fn()
        process.on('unhandledRejection', unhandled)
        try {
            expect(subsonicClient.savePlayQueueBeacon(['tr-1'], 0, 1000)).toBe(true)
            // Node reports unhandled rejections once the current task's
            // microtasks have run; one timer turn is enough to see one.
            await new Promise((resolve) => setTimeout(resolve, 0))
        } finally {
            process.off('unhandledRejection', unhandled)
        }
        expect(unhandled).not.toHaveBeenCalled()
    })

    // The background calls only warn; the warning must not carry the error,
    // whose text would put the apiKey in the console.
    it.each([
        ['scrobble failed', () => subsonicClient.scrobble('tr-1')],
        ['savePlayQueue failed', () => subsonicClient.savePlayQueue(['tr-1'], 0, 0)],
        ['getPlayQueue failed', () => subsonicClient.getPlayQueue()]
    ])('logs "%s" without the failure text', async (label, call) => {
        transport.answer = pluginFailure
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

        await call()

        expect(warn).toHaveBeenCalledWith(label)
        expect(JSON.stringify(warn.mock.calls)).not.toContain('aether_key')
    })
})
