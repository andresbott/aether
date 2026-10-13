import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'

const { getVersion, serverVersion } = vi.hoisted(() => ({
    getVersion: vi.fn(),
    serverVersion: vi.fn()
}))
vi.mock('@/lib/api/Version', () => ({ getVersion }))
vi.mock('@/lib/api/subsonic', () => ({ subsonicClient: { serverVersion } }))

import { useVersion } from '@/composables/useVersion'
import { sessionKind } from '@/lib/host'

function withVersion() {
    let version!: ReturnType<typeof useVersion>
    const Host = defineComponent({
        setup() {
            version = useVersion()
            return () => null
        }
    })
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient: qc }]] } })
    return version
}

beforeEach(() => {
    getVersion.mockReset()
    serverVersion.mockReset()
    sessionKind.value = 'cookie'
})

afterEach(() => {
    sessionKind.value = 'cookie'
})

describe('useVersion', () => {
    it('reads the build info from /api/v0 in a cookie session', async () => {
        const info = { version: '0.4.0', commit: 'abcdef12', build_time: '2026-10-01T00:00:00Z' }
        getVersion.mockResolvedValue(info)
        const version = withVersion()
        await flushPromises()

        expect(version.data.value).toEqual(info)
        expect(serverVersion).not.toHaveBeenCalled()
    })

    // A key session has no /api/v0: the version comes off a /rest ping, and the
    // build details About hides when empty are empty.
    it('reads the version off a /rest ping in a key session', async () => {
        sessionKind.value = 'apiKey'
        serverVersion.mockResolvedValue('0.4.0')
        const version = withVersion()
        await flushPromises()

        expect(version.data.value).toEqual({ version: '0.4.0', commit: '', build_time: '' })
        expect(getVersion).not.toHaveBeenCalled()
    })

    it('reports no version when the ping carries none', async () => {
        sessionKind.value = 'apiKey'
        serverVersion.mockResolvedValue(null)
        const version = withVersion()
        await flushPromises()

        expect(version.data.value?.version).toBe('')
    })
})
