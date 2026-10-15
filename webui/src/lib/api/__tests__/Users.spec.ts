import { describe, it, expect, vi, beforeEach } from 'vitest'

const get = vi.fn()

vi.mock('@/lib/api/client', () => ({
    apiClient: { get: (...a: unknown[]) => get(...a) }
}))

import { getMe } from '@/lib/api/Users'
import { NotAetherMeError } from '@/lib/api/meShape'

beforeEach(() => {
    get.mockReset()
})

describe('Users API', () => {
    // /me is the SPA's bootstrap: a server that silently drops packets must
    // fail it in seconds — the desktop app then offers another server — not
    // leave the window blank for minutes.
    it('bounds the /me bootstrap to ten seconds', async () => {
        const me = { authMethod: 'none', user: null, features: { userManagement: false } }
        get.mockResolvedValue({ data: me })

        await expect(getMe()).resolves.toEqual(me)

        expect(get).toHaveBeenCalledWith('/me', { timeout: 10_000 })
    })

    // In the desktop app a saved address can answer 200 with something else:
    // another web app on that port, a captive portal's page, a proxy's default
    // page. Taken for /me, it would render the app against a server that is not
    // Aether; failing, it brings the server step back — which says that
    // something else answered, rather than that nothing did.
    it.each([
        ['a web page', '<!doctype html><title>Sign in</title>'],
        ['another JSON API', { status: 'ok' }],
        ['an unknown auth method', { authMethod: 'oauth', user: null, features: {} }]
    ])("rejects an answer that is not Aether's: %s", async (_case, data) => {
        get.mockResolvedValue({ data })

        await expect(getMe()).rejects.toBeInstanceOf(NotAetherMeError)
    })
})
