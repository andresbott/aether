import { describe, it, expect, vi, beforeEach } from 'vitest'

const get = vi.fn()

vi.mock('@/lib/api/client', () => ({
    apiClient: { get: (...a: unknown[]) => get(...a) }
}))

import { getMe } from '@/lib/api/Users'

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
})
