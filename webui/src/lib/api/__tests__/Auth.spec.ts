import { describe, it, expect, vi, beforeEach } from 'vitest'

const post = vi.fn()
const put = vi.fn()

vi.mock('@/lib/api/client', () => ({
    apiClient: {
        post: (...a: unknown[]) => post(...a),
        put: (...a: unknown[]) => put(...a)
    }
}))

import * as Auth from '@/lib/api/Auth'

beforeEach(() => {
    post.mockReset()
    put.mockReset()
})

describe('Auth API', () => {
    // Logout is best effort and switching server waits for it: a server that
    // silently drops packets must fail it in seconds, not hold the switch (and the
    // dialog that started it) for minutes. The browser's explicit logout shares the
    // call; its purge runs on settle, so a timeout there only ends the wait.
    it('bounds the logout request to ten seconds and revokes the given spa token', async () => {
        post.mockResolvedValue({ data: null })

        await Auth.logout('spa-token-1')

        expect(post).toHaveBeenCalledWith(
            '/auth/logout',
            { tokenId: 'spa-token-1' },
            { timeout: 10_000 }
        )
    })

    it('bounds the logout request just the same when there is no spa token to revoke', async () => {
        post.mockResolvedValue({ data: null })

        await Auth.logout()

        expect(post).toHaveBeenCalledWith('/auth/logout', undefined, { timeout: 10_000 })
    })

    it('changes the password by PUT-ing the current and new values', async () => {
        put.mockResolvedValue({ data: null })
        await Auth.changePassword('old-pw', 'new-pw')
        expect(put).toHaveBeenCalledWith('/auth/password', {
            currentPassword: 'old-pw',
            newPassword: 'new-pw'
        })
    })
})
