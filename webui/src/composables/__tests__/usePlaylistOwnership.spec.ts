import { describe, it, expect, vi } from 'vitest'
import { ref } from 'vue'

const currentUser = ref<{ login: string; role: string } | null>(null)
vi.mock('@/composables/useAuth', () => ({ useAuth: () => ({ currentUser }) }))

import { useIsPlaylistOwner } from '@/composables/usePlaylistOwnership'

describe('useIsPlaylistOwner', () => {
    it('treats a null identity (auth "none") as owner', () => {
        currentUser.value = null
        expect(useIsPlaylistOwner(() => 'admin').value).toBe(true)
    })

    it('is owner when the login matches the playlist owner', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        expect(useIsPlaylistOwner(() => 'alice').value).toBe(true)
    })

    it('is not owner when the login differs from the playlist owner', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        expect(useIsPlaylistOwner(() => 'bob').value).toBe(false)
    })
})
