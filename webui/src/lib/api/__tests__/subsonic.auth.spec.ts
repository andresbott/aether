import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'
import { sessionExpired } from '@/lib/authState'

const failedEnvelope = (code: number) => ({
    ok: true,
    json: async () => ({
        'subsonic-response': {
            status: 'failed',
            version: '1.16.1',
            error: { code, message: 'authentication required' }
        }
    })
})

describe('subsonic client session expiry', () => {
    beforeEach(() => {
        sessionExpired.value = false
        subsonicClient.initWithDefaults()
    })
    afterEach(() => {
        vi.restoreAllMocks()
        sessionExpired.value = false
    })

    it('flips sessionExpired when /rest answers subsonic error 40', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(failedEnvelope(40)))
        await expect(subsonicClient.getPlaylists()).rejects.toThrow()
        expect(sessionExpired.value).toBe(true)
    })

    it('leaves sessionExpired alone on other subsonic errors', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(failedEnvelope(70)))
        await expect(subsonicClient.getPlaylists()).rejects.toThrow()
        expect(sessionExpired.value).toBe(false)
    })
})
