import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import type { ScanFolder } from '@/types/scanFolders'

const listScanFoldersMock = vi.fn()

vi.mock('@/lib/api/ScanFolders', () => ({
    listScanFolders: (...args: unknown[]) => listScanFoldersMock(...args)
}))

import { useScanFolders } from '@/composables/useScanFolders'

const folders: ScanFolder[] = [
    {
        name: 'Music',
        path: '/srv/music',
        exclude_patterns: [],
        follow_symlinks: true,
        available: true,
        track_count: 12
    },
    {
        name: 'Offline',
        path: '/mnt/share',
        exclude_patterns: ['^\\.'],
        follow_symlinks: false,
        available: false,
        problem: 'root "/mnt/share" is unavailable',
        track_count: 0
    }
]

beforeEach(() => {
    listScanFoldersMock.mockReset()
})

describe('useScanFolders', () => {
    it('lists the configured scan folders once', async () => {
        listScanFoldersMock.mockResolvedValue(folders)
        let query!: ReturnType<typeof useScanFolders>
        const Comp = defineComponent({
            setup() {
                query = useScanFolders()
                return () => h('div')
            }
        })
        mount(Comp, {
            global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient() }]] }
        })
        await flushPromises()

        expect(listScanFoldersMock).toHaveBeenCalledTimes(1)
        expect(query.data.value).toEqual(folders)
    })
})
