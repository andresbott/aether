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

const genresSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Artists', () => ({
    getReleaseGroupGenres: genresSpy
}))

const stubs = {
    Dialog: { template: '<div><slot /><slot name="footer" /></div>' },
    InputText: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
    Button: { props: ['label', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>' },
    Checkbox: {
        props: ['modelValue', 'inputId'],
        template:
            '<input type="checkbox" :id="inputId" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />'
    },
    Message: { template: '<div><slot /></div>' }
}

const defaultProps = {
    visible: true,
    albumName: 'OK',
    currentReleaseMbid: '',
    currentReleaseGroupMbid: '',
    currentYear: 0,
    currentAlbumArtists: [] as { name: string; mbid: string }[],
    currentGenres: [] as string[]
}

const mkCandidate = (over: Record<string, any> = {}) => ({
    releaseMbid: 'rel-1',
    releaseGroupMbid: 'rg-1',
    title: 'OK Computer',
    artist: 'Radiohead',
    artists: [{ name: 'Radiohead', mbid: 'artist-1' }],
    date: '1997-05-21',
    country: 'GB',
    trackCount: 12,
    disambiguation: '',
    score: 100,
    ...over
})

describe('MusicBrainzAlbumPicker', () => {
    beforeEach(() => {
        searchSpy.mockClear()
        genresSpy.mockReset()
        genresSpy.mockResolvedValue([])
        mbState.results.value = []
        mbState.loading.value = false
        mbState.error.value = null
    })

    it('stages a search result and, on OK, emits the full payload', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: defaultProps,
            global: { stubs }
        })
        // Clicking a result only stages it — the dialog does not close yet.
        await wrapper.find('.result-row').trigger('click')
        expect(wrapper.emitted('select')).toBeUndefined()
        expect(wrapper.find('[data-test="album-mbid-confirm"]').attributes('disabled')).toBeUndefined()
        await wrapper.find('[data-test="album-mbid-confirm"]').trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual([
            {
                album: 'OK Computer',
                year: 1997,
                albumArtists: [{ name: 'Radiohead', mbid: 'artist-1' }],
                mbReleaseId: 'rel-1',
                mbReleaseGroupId: 'rg-1'
            }
        ])
    })

    it('shows a preview row per field with current → new', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: {
                ...defaultProps,
                albumName: 'OK',
                currentYear: 1996,
                currentAlbumArtists: [{ name: 'Radio Head', mbid: '' }]
            },
            global: { stubs }
        })
        expect(wrapper.find('[data-test="album-preview"]').exists()).toBe(false)
        await wrapper.find('.result-row').trigger('click')
        expect(wrapper.find('[data-test="album-preview"]').exists()).toBe(true)
        expect(wrapper.find('[data-test="preview-album"]').text()).toContain('OK Computer')
        const year = wrapper.find('[data-test="preview-year"]')
        expect(year.text()).toContain('1996')
        expect(year.text()).toContain('1997')
        const artists = wrapper.find('[data-test="preview-albumArtists"]')
        expect(artists.text()).toContain('Radio Head')
        expect(artists.text()).toContain('Radiohead')
        expect(wrapper.find('[data-test="preview-mbReleaseId"]').text()).toContain('rel-1')
        expect(wrapper.find('[data-test="preview-mbReleaseGroupId"]').text()).toContain('rg-1')
    })

    it('marks fields whose value would not change as unchanged', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: { ...defaultProps, albumName: 'OK Computer' },
            global: { stubs }
        })
        await wrapper.find('.result-row').trigger('click')
        expect(wrapper.find('[data-test="preview-album"]').text()).toContain('(unchanged)')
        expect(wrapper.find('[data-test="preview-year"]').text()).not.toContain('(unchanged)')
    })

    it('omits unchecked fields from the payload', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: defaultProps,
            global: { stubs }
        })
        await wrapper.find('.result-row').trigger('click')
        await wrapper.find('[data-test="preview-year"] input[type="checkbox"]').setValue(false)
        await wrapper
            .find('[data-test="preview-albumArtists"] input[type="checkbox"]')
            .setValue(false)
        await wrapper.find('[data-test="album-mbid-confirm"]').trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual([
            {
                album: 'OK Computer',
                mbReleaseId: 'rel-1',
                mbReleaseGroupId: 'rg-1'
            }
        ])
    })

    it('disables OK when every field is unchecked', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: defaultProps,
            global: { stubs }
        })
        await wrapper.find('.result-row').trigger('click')
        for (const box of wrapper.findAll('.preview-row input[type="checkbox"]')) {
            await box.setValue(false)
        }
        expect(wrapper.find('[data-test="album-mbid-confirm"]').attributes('disabled')).toBeDefined()
    })

    it('omits the year row when the candidate has no date', async () => {
        mbState.results.value = [mkCandidate({ date: '' })]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: defaultProps,
            global: { stubs }
        })
        await wrapper.find('.result-row').trigger('click')
        expect(wrapper.find('[data-test="preview-year"]').exists()).toBe(false)
        await wrapper.find('[data-test="album-mbid-confirm"]').trigger('click')
        const payload = wrapper.emitted('select')?.[0]?.[0] as Record<string, unknown>
        expect(payload.year).toBeUndefined()
    })

    it('offers looked-up genres as a preview row and emits them on OK', async () => {
        genresSpy.mockResolvedValue(['alternative rock', 'art rock'])
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: { ...defaultProps, currentGenres: ['rock'] },
            global: { stubs }
        })
        await wrapper.find('.result-row').trigger('click')
        await vi.waitFor(() =>
            expect(wrapper.find('[data-test="preview-genres"]').exists()).toBe(true)
        )
        expect(genresSpy).toHaveBeenCalledWith('rg-1')
        const row = wrapper.find('[data-test="preview-genres"]')
        expect(row.text()).toContain('rock')
        expect(row.text()).toContain('alternative rock, art rock')
        await wrapper.find('[data-test="album-mbid-confirm"]').trigger('click')
        const payload = wrapper.emitted('select')?.[0]?.[0] as Record<string, unknown>
        expect(payload.genres).toEqual(['alternative rock', 'art rock'])
    })

    it('hides the genres row when the lookup returns nothing', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: defaultProps,
            global: { stubs }
        })
        await wrapper.find('.result-row').trigger('click')
        await vi.waitFor(() => expect(genresSpy).toHaveBeenCalled())
        expect(wrapper.find('[data-test="preview-genres"]').exists()).toBe(false)
        await wrapper.find('[data-test="album-mbid-confirm"]').trigger('click')
        const payload = wrapper.emitted('select')?.[0]?.[0] as Record<string, unknown>
        expect(payload.genres).toBeUndefined()
    })

    it('OK stays disabled until a result is picked', async () => {
        mbState.results.value = [mkCandidate()]
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: defaultProps,
            global: { stubs }
        })
        expect(wrapper.find('[data-test="album-mbid-confirm"]').attributes('disabled')).toBeDefined()
    })

    it('clears both IDs when the linked match is cleared', async () => {
        const wrapper = mount(MusicBrainzAlbumPicker, {
            props: {
                ...defaultProps,
                currentReleaseMbid: 'rel-1',
                currentReleaseGroupMbid: 'rg-1'
            },
            global: { stubs }
        })
        await wrapper.find('.clear-btn').trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual([{ mbReleaseId: '', mbReleaseGroupId: '' }])
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
                props: defaultProps,
                global: { stubs }
            })
            await vi.advanceTimersByTimeAsync(500)
            expect(searchSpy).toHaveBeenCalledTimes(1)
        })
    })
})
