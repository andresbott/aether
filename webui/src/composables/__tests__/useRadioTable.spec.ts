import { describe, it, expect, vi } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'

const stations = ref<Array<{ id: string; name: string; streamUrl: string }>>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    useRadioStations: () => ({ data: stations, isLoading: ref(false), error: ref(null) })
}))

import { useRadioTable } from '@/composables/useRadioTable'

function withComposable() {
    const captured: { api?: ReturnType<typeof useRadioTable> } = {}
    const Host = defineComponent({
        setup() {
            captured.api = useRadioTable()
            return () => h('div')
        }
    })
    mount(Host)
    return captured
}

describe('useRadioTable', () => {
    it('exposes total and items sorted by name (case-insensitive)', () => {
        stations.value = [
            { id: 's1', name: 'beta', streamUrl: 'b' },
            { id: 's2', name: 'Alpha', streamUrl: 'a' }
        ]
        const c = withComposable()
        expect(c.api!.total.value).toBe(2)
        expect(c.api!.items.value.map((s) => s.name)).toEqual(['Alpha', 'beta'])
    })

    it('reports zero total when there are no stations', () => {
        stations.value = []
        const c = withComposable()
        expect(c.api!.total.value).toBe(0)
        expect(c.api!.items.value).toEqual([])
    })
})
