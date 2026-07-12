import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import MusicBrainzAlbumPicker from '@/components/library/MusicBrainzAlbumPicker.vue'

const searchSpy = vi.hoisted(() => vi.fn())
// Fake refs (__v_isRef) so Vue's template unwraps `.value`, matching the real
// composable's return shape. Tests mutate `.value` to drive render state.
const mbState = vi.hoisted(() => ({
    results: { __v_isRef: true, value: [] as any[] },
    loading: { __v_isRef: true, value: false },
    error: { __v_isRef: true, value: null as string | null }
}))

vi.mock('@/composables/useMusicBrainzReleaseSearch', () => ({
    useMusicBrainzReleaseSearch: () => ({
        results: mbState.results,
        loading: mbState.loading,
        error: mbState.error,
        search: searchSpy
    })
}))

const stubs = {
    Dialog: { template: '<div><slot /><slot name="footer" /></div>' },
    InputText: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
    Button: { props: ['label', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>' },
    Message: { template: '<div><slot /></div>' }
}

const mkCandidate = (over: Record<string, any> = {}) => ({
    releaseMbid: 'rel-1',
    releaseGroupMbid: 'rg-1',
    title: 'OK Computer',
    artist: 'Radiohead',
    date: '1997',
    country: 'GB',
    trackCount: 12,
    disambiguation: '',
    score: 100,
    ...over
})

describe('MusicBrainzAlbumPicker', () => {
    beforeEach(() => {
        searchSpy.mockClear()
        mbState.results.value = []
        mbState.loading.value = false
        mbState.error.value = null
    })

    it('stages a search result and, on OK, emits both IDs and the title', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: {
                visible: true,
                albumName: 'OK',
                currentReleaseMbid: '',
                currentReleaseGroupMbid: ''
            },
            global: { stubs }
        })
        // Clicking a result only stages it — the dialog does not close yet.
        await wrapper.find('.result-row').trigger('click')
        expect(wrapper.emitted('select')).toBeUndefined()
        expect(wrapper.find('[data-test="album-mbid-confirm"]').attributes('disabled')).toBeUndefined()
        await wrapper.find('[data-test="album-mbid-confirm"]').trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual(['rel-1', 'rg-1', 'OK Computer'])
    })

    it('OK stays disabled until a result is picked', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: {
                visible: true,
                albumName: 'OK',
                currentReleaseMbid: '',
                currentReleaseGroupMbid: ''
            },
            global: { stubs }
        })
        expect(wrapper.find('[data-test="album-mbid-confirm"]').attributes('disabled')).toBeDefined()
    })

    it('clears both IDs when the linked match is cleared', async () => {
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: {
                visible: true,
                albumName: 'OK',
                currentReleaseMbid: 'rel-1',
                currentReleaseGroupMbid: 'rg-1'
            },
            global: { stubs }
        })
        await wrapper.find('.clear-btn').trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual(['', ''])
    })

    describe('search on open', () => {
        beforeEach(() => {
            vi.useFakeTimers()
        })

        afterEach(() => {
            vi.useRealTimers()
        })

        it('searches exactly once when the dialog opens', async () => {
            mount(MusicBrainzAlbumPicker, {
                props: {
                    visible: true,
                    albumName: 'OK',
                    currentReleaseMbid: '',
                    currentReleaseGroupMbid: ''
                },
                global: { stubs }
            })
            await vi.advanceTimersByTimeAsync(500)
            expect(searchSpy).toHaveBeenCalledTimes(1)
        })
    })
})
