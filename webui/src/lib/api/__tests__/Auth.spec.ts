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
    it('changes the password by PUT-ing the current and new values', async () => {
        put.mockResolvedValue({ data: null })
        await Auth.changePassword('old-pw', 'new-pw')
        expect(put).toHaveBeenCalledWith('/auth/password', {
            currentPassword: 'old-pw',
            newPassword: 'new-pw'
        })
    })
})
