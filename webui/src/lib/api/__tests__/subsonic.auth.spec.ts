import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'

const okPing = {
    'subsonic-response': { status: 'ok', version: '1.16.1' }
}

describe('subsonic client apiKey auth', () => {
    beforeEach(() => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve(okPing)
        }))
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        subsonicClient.clearApiKey()
    })

    it('appends apiKey and no u/t/s/p params when a key is set', async () => {
        subsonicClient.setApiKey('aether_abc_secret')
        await subsonicClient.ping()
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.get('apiKey')).toBe('aether_abc_secret')
        for (const p of ['u', 't', 's', 'p']) {
            expect(url.searchParams.has(p), `param ${p}`).toBe(false)
        }
    })

    it('sends no auth params in auth-none mode', async () => {
        subsonicClient.initWithDefaults()
        await subsonicClient.ping()
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.has('apiKey')).toBe(false)
        expect(url.searchParams.has('u')).toBe(false)
    })

    it('getStreamUrl carries the apiKey', () => {
        subsonicClient.setApiKey('aether_abc_secret')
        const url = new URL(subsonicClient.getStreamUrl('tr-1'))
        expect(url.searchParams.get('apiKey')).toBe('aether_abc_secret')
    })
})
