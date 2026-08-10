import { describe, it, expect, vi, beforeEach } from 'vitest'

const get = vi.fn()
const post = vi.fn()
const del = vi.fn()

vi.mock('@/lib/api/client', () => ({
    apiClient: {
        get: (...a: unknown[]) => get(...a),
        post: (...a: unknown[]) => post(...a),
        delete: (...a: unknown[]) => del(...a)
    }
}))

import * as Tokens from '@/lib/api/Tokens'
import { getDeviceId, getDeviceName } from '@/lib/deviceIdentity'

beforeEach(() => {
    get.mockReset()
    post.mockReset()
    del.mockReset()
})

describe('Tokens API', () => {
    // The server supersedes only the session bearing the same deviceId, so a
    // mint without one would 400 — and one with the wrong one would sign
    // another browser out.
    it('mints with this browser identity so other browsers keep their sessions', async () => {
        post.mockResolvedValue({
            data: { token: 'aether_k', tokenId: 'tid1', expiresAt: '2999-01-01T00:00:00Z' }
        })

        const res = await Tokens.mintSpaToken()

        expect(post).toHaveBeenCalledWith('/auth/token', {
            deviceId: getDeviceId(),
            deviceName: getDeviceName()
        })
        expect(res.tokenId).toBe('tid1')
    })

    it('lists the caller tokens and tolerates an absent tokens array', async () => {
        get.mockResolvedValue({ data: { tokens: [{ tokenId: 'a' }] } })
        expect(await Tokens.listTokens()).toEqual([{ tokenId: 'a' }])

        get.mockResolvedValue({ data: {} })
        expect(await Tokens.listTokens()).toEqual([])
    })

    it('creates a token with the requested name and type', async () => {
        post.mockResolvedValue({ data: { tokenId: 'tid2', token: 'aether_x' } })
        await Tokens.createToken({ name: 'phone', type: 'usertoken' })
        expect(post).toHaveBeenCalledWith('/auth/tokens', { name: 'phone', type: 'usertoken' })
    })

    it('escapes the token id when revoking', async () => {
        del.mockResolvedValue({ data: null })
        await Tokens.revokeToken('a/b')
        expect(del).toHaveBeenCalledWith('/auth/tokens/a%2Fb')
    })
})
