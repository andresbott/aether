import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const mockGetGeneratedCoverCandidates = vi.fn()
const mockGetGeneratedCoverPreviewUrl = vi.fn()

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        getGeneratedCoverCandidates: (...args: unknown[]) => mockGetGeneratedCoverCandidates(...args),
        getGeneratedCoverPreviewUrl: (...args: unknown[]) => mockGetGeneratedCoverPreviewUrl(...args)
    }
}))

import GenerateCoverDialog from '../GenerateCoverDialog.vue'

const stubs = {
    Dialog: { template: '<div><slot /><slot name="footer" /></div>' },
    Button: {
        props: ['label', 'icon', 'text', 'loading'],
        emits: ['click'],
        template: '<button :aria-label="label" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

const mountDialog = (props = {}) =>
    mount(GenerateCoverDialog, {
        props: {
            visible: false,
            entityId: 'entity1',
            title: 'Test Entity',
            ...props
        },
        global: {
            plugins: [PrimeVue],
            stubs
        }
    })

beforeEach(() => {
    mockGetGeneratedCoverCandidates.mockClear()
    mockGetGeneratedCoverPreviewUrl.mockClear()
    mockGetGeneratedCoverCandidates.mockResolvedValue([
        { style: 'bauhaus', variation: 1 },
        { style: 'rings', variation: 2 }
    ])
    mockGetGeneratedCoverPreviewUrl.mockImplementation(
        ({ style, variation }: { style: string; variation: number }) =>
            `preview:${style}:${variation}`
    )
})

describe('GenerateCoverDialog', () => {
    it('loads candidates when opened', async () => {
        const w = mountDialog({ visible: false })
        expect(mockGetGeneratedCoverCandidates).not.toHaveBeenCalled()

        await w.setProps({ visible: true })
        await flushPromises()

        expect(mockGetGeneratedCoverCandidates).toHaveBeenCalledWith('entity1', 9)
    })

    it('renders a thumbnail per candidate', async () => {
        const w = mountDialog({ visible: true })
        await flushPromises()

        // PrimeVue Dialog needs an extra tick to render
        await w.vm.$nextTick()

        const thumbnails = w.findAll('.candidate')
        expect(thumbnails).toHaveLength(2)

        const images = w.findAll('.candidate img')
        expect(images[0].attributes('src')).toBe('preview:bauhaus:1')
        expect(images[1].attributes('src')).toBe('preview:rings:2')
    })

    it('emits select and closes when a candidate is clicked', async () => {
        const w = mountDialog({ visible: true })
        await flushPromises()
        await w.vm.$nextTick()

        const thumbnails = w.findAll('.candidate')
        await thumbnails[0].trigger('click')

        expect(w.emitted('select')).toHaveLength(1)
        expect(w.emitted('select')![0]).toEqual([{ style: 'bauhaus', variation: 1 }])
        expect(w.emitted('update:visible')).toHaveLength(1)
        expect(w.emitted('update:visible')![0]).toEqual([false])
    })

    it('re-calls getGeneratedCoverCandidates when Shuffle is clicked', async () => {
        const w = mountDialog({ visible: true })
        await flushPromises()
        await w.vm.$nextTick()

        expect(mockGetGeneratedCoverCandidates).toHaveBeenCalledTimes(1)

        const shuffle = w.find('button[aria-label="Shuffle"]')
        await shuffle.trigger('click')
        await flushPromises()

        expect(mockGetGeneratedCoverCandidates).toHaveBeenCalledTimes(2)
        expect(mockGetGeneratedCoverCandidates).toHaveBeenLastCalledWith('entity1', 9)
    })

    it('emits update:visible false when Cancel is clicked', async () => {
        const w = mountDialog({ visible: true })
        await flushPromises()
        await w.vm.$nextTick()

        const cancel = w.find('button[aria-label="Cancel"]')
        await cancel.trigger('click')

        expect(w.emitted('update:visible')).toHaveLength(1)
        expect(w.emitted('update:visible')![0]).toEqual([false])
    })
})
