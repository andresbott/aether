import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import RawEditPanel from '@/views/settings/metadata-editor/RawEditPanel.vue'
import { useEditSession, type EditSession } from '@/composables/useEditSession'
import type { RawTagsResult, Track } from '@/types/metadata'

// The session composable needs vue-query/toast at setup; stub the
// side-effecting pieces (pattern shared with EditPanel.spec.ts).
// vi.hoisted runs before imports, so the holder is a plain object exposed to
// the component through reactive refs created inside mountRaw.
const rawTagsData = vi.hoisted(() => ({ current: undefined as unknown }))
vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useRawTags: () => ({
            data: rawTagsData.current,
            isLoading: { __v_isRef: true, value: false }
        }),
        useApplyPicture: () => ({ mutateAsync: vi.fn(), isPending: { __v_isRef: true, value: false } }),
        useDeletePicture: () => ({ mutateAsync: vi.fn(), isPending: { __v_isRef: true, value: false } })
    }
})
vi.mock('@tanstack/vue-query', async (importActual) => {
    const actual = await importActual<typeof import('@tanstack/vue-query')>()
    return {
        ...actual,
        useQueryClient: () => ({ invalidateQueries: vi.fn() })
    }
})
vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: vi.fn() })
}))

const stubs = {
    InputText: {
        props: ['modelValue'],
        template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Textarea: {
        props: ['modelValue', 'disabled', 'placeholder'],
        template:
            '<textarea :disabled="disabled" :placeholder="placeholder" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :aria-label="$attrs[\'aria-label\']" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: '',
    artists: [],
    album_artists: [],
    album: '',
    genres: [],
    year: 0,
    track_number: 0,
    disc_number: 0,
    disc_subtitle: '',
    compilation: false,
    mb_artist_ids: [],
    mb_album_artist_ids: [],
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: '',
    ...over
})

function mountRaw(selection: Track[], results: RawTagsResult[]) {
    rawTagsData.current = ref(results)
    const session = useEditSession(
        () => selection,
        () => 1
    )
    const wrapper = mount(RawEditPanel, {
        props: { selection, libraryId: 1, session },
        global: { stubs, directives: { tooltip: {} } }
    })
    return { wrapper, session }
}

function rawPatch(session: EditSession, path: string) {
    return session.overlays.value.get(path)?.raw
}

beforeEach(() => {
    rawTagsData.current = ref(undefined)
})

describe('RawEditPanel', () => {
    const track = mkTrack()
    const singleResult: RawTagsResult[] = [
        {
            path: 'a.mp3',
            tags: {
                TITLE: ['Song'],
                CUSTOM: ['x', 'y'],
                REPLAYGAIN_TRACK_GAIN: ['-3.1 dB']
            },
            unsupported: []
        }
    ]

    it('renders every key sorted, with managed keys locked', () => {
        const { wrapper } = mountRaw([track], singleResult)
        const rows = wrapper.findAll('[data-test^="raw-row-"]')
        expect(rows.map((r) => r.attributes('data-test'))).toEqual([
            'raw-row-CUSTOM',
            'raw-row-REPLAYGAIN_TRACK_GAIN',
            'raw-row-TITLE'
        ])
        const titleRow = wrapper.find('[data-test="raw-row-TITLE"]')
        expect(titleRow.find('[data-test="raw-managed"]').exists()).toBe(true)
        expect(titleRow.find('textarea').attributes('disabled')).toBeDefined()
        expect(titleRow.find('[data-test="raw-delete-TITLE"]').exists()).toBe(false)
        // Non-managed rows are editable with a delete button.
        const customRow = wrapper.find('[data-test="raw-row-CUSTOM"]')
        expect(customRow.find('textarea').attributes('disabled')).toBeUndefined()
        expect(customRow.find('[data-test="raw-delete-CUSTOM"]').exists()).toBe(true)
    })

    it('shows multi-values one per line and stages edits', async () => {
        const { wrapper, session } = mountRaw([track], singleResult)
        const textarea = wrapper.find('[data-test="raw-row-CUSTOM"] textarea')
        expect((textarea.element as HTMLTextAreaElement).value).toBe('x\ny')
        await textarea.setValue('x\nz')
        expect(rawPatch(session, 'a.mp3')).toEqual({ CUSTOM: ['x', 'z'] })
    })

    it('typing the original value back clears the staged key', async () => {
        const { wrapper, session } = mountRaw([track], singleResult)
        const textarea = wrapper.find('[data-test="raw-row-CUSTOM"] textarea')
        await textarea.setValue('changed')
        expect(session.hasStagedChanges.value).toBe(true)
        await textarea.setValue('x\ny')
        expect(session.hasStagedChanges.value).toBe(false)
    })

    it('delete stages an empty value list and shows the deleted note', async () => {
        const { wrapper, session } = mountRaw([track], singleResult)
        await wrapper.find('[data-test="raw-delete-CUSTOM"]').trigger('click')
        expect(rawPatch(session, 'a.mp3')).toEqual({ CUSTOM: [] })
        expect(wrapper.find('[data-test="raw-delete-note"]').exists()).toBe(true)
        // Revert restores the row.
        await wrapper.find('[data-test="raw-undo-CUSTOM"]').trigger('click')
        expect(session.hasStagedChanges.value).toBe(false)
        expect(wrapper.find('[data-test="raw-delete-note"]').exists()).toBe(false)
    })

    it('adds a new tag on all selected tracks', async () => {
        const b = mkTrack({ path: 'b.mp3' })
        const { wrapper, session } = mountRaw(
            [track, b],
            [
                { path: 'a.mp3', tags: { CUSTOM: ['x'] }, unsupported: [] },
                { path: 'b.mp3', tags: {}, unsupported: [] }
            ]
        )
        await wrapper.find('[data-test="raw-add-key"]').setValue('mood')
        await wrapper.find('[data-test="raw-add-value"]').setValue('calm')
        await wrapper.find('[data-test="raw-add-button"]').trigger('click')
        expect(rawPatch(session, 'a.mp3')).toEqual({ MOOD: ['calm'] })
        expect(rawPatch(session, 'b.mp3')).toEqual({ MOOD: ['calm'] })
        // The new key now renders as a row.
        expect(wrapper.find('[data-test="raw-row-MOOD"]').exists()).toBe(true)
    })

    it('rejects adding a managed or duplicate key', async () => {
        const { wrapper } = mountRaw([track], singleResult)
        await wrapper.find('[data-test="raw-add-key"]').setValue('ARTIST')
        expect(wrapper.find('[data-test="raw-add-error"]').text()).toContain('managed')
        await wrapper.find('[data-test="raw-add-key"]').setValue('CUSTOM')
        expect(wrapper.find('[data-test="raw-add-error"]').text()).toContain('already exists')
    })

    it('marks differing values across tracks as mixed', () => {
        const b = mkTrack({ path: 'b.mp3' })
        const { wrapper } = mountRaw(
            [track, b],
            [
                { path: 'a.mp3', tags: { CUSTOM: ['x'] }, unsupported: [] },
                { path: 'b.mp3', tags: { CUSTOM: ['y'] }, unsupported: [] }
            ]
        )
        const textarea = wrapper.find('[data-test="raw-row-CUSTOM"] textarea')
        expect((textarea.element as HTMLTextAreaElement).value).toBe('')
        expect(textarea.attributes('placeholder')).toContain('multiple values')
    })

    it('overwrites all tracks when editing a mixed key', async () => {
        const b = mkTrack({ path: 'b.mp3' })
        const { wrapper, session } = mountRaw(
            [track, b],
            [
                { path: 'a.mp3', tags: { CUSTOM: ['x'] }, unsupported: [] },
                { path: 'b.mp3', tags: { CUSTOM: ['y'] }, unsupported: [] }
            ]
        )
        await wrapper.find('[data-test="raw-row-CUSTOM"] textarea').setValue('same')
        expect(rawPatch(session, 'a.mp3')).toEqual({ CUSTOM: ['same'] })
        expect(rawPatch(session, 'b.mp3')).toEqual({ CUSTOM: ['same'] })
    })

    it('lists hidden frames and stages their deletion per carrying track', async () => {
        const b = mkTrack({ path: 'b.mp3' })
        const { wrapper, session } = mountRaw(
            [track, b],
            [
                {
                    path: 'a.mp3',
                    tags: {},
                    unsupported: ['PRIV/com.example.junk', 'GEOB']
                },
                { path: 'b.mp3', tags: {}, unsupported: ['GEOB'] }
            ]
        )
        const rows = wrapper.findAll('[data-test^="raw-hidden-row-"]')
        expect(rows.map((r) => r.attributes('data-test'))).toEqual([
            'raw-hidden-row-GEOB',
            'raw-hidden-row-PRIV/com.example.junk'
        ])
        // GEOB exists on both tracks, PRIV only on a.mp3.
        expect(wrapper.find('[data-test="raw-hidden-row-GEOB"]').text()).toContain('2 of 2')
        expect(
            wrapper.find('[data-test="raw-hidden-row-PRIV/com.example.junk"]').text()
        ).toContain('1 of 2')

        // Deleting GEOB stages it on both tracks.
        await wrapper.find('[data-test="raw-hidden-delete-GEOB"]').trigger('click')
        expect(session.overlays.value.get('a.mp3')?.removeUnsupported).toEqual(['GEOB'])
        expect(session.overlays.value.get('b.mp3')?.removeUnsupported).toEqual(['GEOB'])

        // Deleting PRIV stages it only on the track that carries it.
        await wrapper
            .find('[data-test="raw-hidden-delete-PRIV/com.example.junk"]')
            .trigger('click')
        expect(session.overlays.value.get('a.mp3')?.removeUnsupported).toEqual([
            'GEOB',
            'PRIV/com.example.junk'
        ])
        expect(session.overlays.value.get('b.mp3')?.removeUnsupported).toEqual(['GEOB'])
        expect(wrapper.findAll('[data-test="raw-hidden-delete-note"]').length).toBe(2)

        // Undo removes the staging again.
        await wrapper.find('[data-test="raw-hidden-undo-GEOB"]').trigger('click')
        expect(session.overlays.value.get('a.mp3')?.removeUnsupported).toEqual([
            'PRIV/com.example.junk'
        ])
        expect(session.overlays.value.get('b.mp3')).toBeUndefined()
    })

    it('renders no hidden-frames section when the selection carries none', () => {
        const { wrapper } = mountRaw([track], singleResult)
        expect(wrapper.find('[data-test="raw-hidden"]').exists()).toBe(false)
    })

    it('shows per-path read errors', () => {
        const { wrapper } = mountRaw(
            [track],
            [{ path: 'a.mp3', tags: {}, unsupported: [], error: 'unsupported format' }]
        )
        expect(wrapper.text()).toContain('unsupported format')
    })
})
