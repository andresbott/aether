import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import MetadataEditorView from '@/views/settings/MetadataEditorView.vue'
import { useIdentifyCache } from '@/composables/useIdentifyCache'
import type { IdentifyAlbumResponse, IdentifyTrackResult, Track } from '@/types/metadata'

// The view is wired to the identify cache through useIdentifyRuns; these tests
// assert the wiring (a reopened dialog costs no request, Reload forgets), so
// only the pieces the view needs at setup are stubbed.
const identifySpy = vi.hoisted(() => vi.fn())
const identifyAlbumSpy = vi.hoisted(() => vi.fn())
const refetchSpy = vi.hoisted(() => vi.fn())
const tracksRef = vi.hoisted(() => ({ value: [] as Track[] }))

vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useTracks: () => ({
            data: tracksRef,
            isLoading: { value: false },
            refetch: refetchSpy
        }),
        useMetadataCapabilities: () => ({
            data: { value: { identify: true } },
            isPending: { value: false }
        }),
        useIdentifyTracks: () => ({ mutateAsync: identifySpy, isPending: { value: false } }),
        useIdentifyAlbum: () => ({ mutateAsync: identifyAlbumSpy, isPending: { value: false } }),
        useApplyPicture: () => ({ mutateAsync: vi.fn() }),
        useDeletePicture: () => ({ mutateAsync: vi.fn() })
    }
})
vi.mock('@/composables/useLibraries', () => ({
    useLibraries: () => ({ data: { value: [{ id: 1, name: 'Music' }] } })
}))
vi.mock('@tanstack/vue-query', async (importActual) => {
    const actual = await importActual<typeof import('@tanstack/vue-query')>()
    return { ...actual, useQueryClient: () => ({ invalidateQueries: vi.fn() }) }
})
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('primevue/useconfirm', () => ({
    // Every guard in this view runs through confirm.require; accept immediately
    // so a test can reach the action behind it.
    useConfirm: () => ({ require: (opts: { accept?: () => void }) => opts.accept?.() })
}))
vi.mock('vue-router', () => ({ onBeforeRouteLeave: () => {} }))

const mkTrack = (path: string): Track => ({
    path,
    name: path,
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
    mb_release_group_id: ''
})

const mkResult = (path: string): IdentifyTrackResult => ({
    path,
    candidates: [
        { recording_mbid: `rec-${path}`, title: 'Found', artists: [], score: 0.95, releases: [] }
    ]
})

const albumResponse: IdentifyAlbumResponse = {
    options: [
        {
            release_mbid: 'rel-A',
            release_group_mbid: 'rg-A',
            album: 'Album A',
            year: 1991,
            artists: [],
            track_count: 2,
            disc_count: 1,
            enriched: true,
            matched_count: 2,
            mean_score: 0.9,
            assignments: [],
            tracks: []
        }
    ],
    errors: []
}

const tracks = [mkTrack('a.mp3'), mkTrack('b.mp3')]

// The child components are exercised by their own specs; here they are reduced
// to the events the view has to react to.
const stubs = {
    Dialog: {
        name: 'Dialog',
        props: ['visible'],
        template: '<div v-if="visible"><slot /><slot name="footer" /></div>'
    },
    ConfirmDialog: true,
    Dropdown: {
        name: 'Dropdown',
        // options is declared so it does not fall through to the <select> as a
        // DOM prop, which jsdom refuses to set.
        props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'placeholder'],
        emits: ['update:modelValue'],
        template: '<select />'
    },
    Splitter: { name: 'Splitter', template: '<div><slot /></div>' },
    SplitterPanel: { name: 'SplitterPanel', template: '<div><slot /></div>' },
    Button: {
        name: 'Button',
        props: ['label'],
        inheritAttrs: false,
        template:
            '<button :data-test="$attrs[\'data-test\']" :aria-label="$attrs[\'aria-label\']" @click="$emit(\'click\')">{{ label }}</button>'
    },
    FolderTree: { name: 'FolderTree', template: '<div />' },
    TrackList: { name: 'TrackList', template: '<div />' },
    EditPanel: {
        name: 'EditPanel',
        props: ['selection'],
        template: '<div class="edit-panel-stub" />'
    },
    IdentifyReviewDialog: {
        name: 'IdentifyReviewDialog',
        props: ['visible', 'results', 'loading'],
        template: '<div class="review-dialog-stub" />'
    },
    IdentifyAlbumDialog: {
        name: 'IdentifyAlbumDialog',
        props: ['visible', 'options', 'loading'],
        template: '<div class="album-dialog-stub" />'
    }
}

function mountView() {
    return mount(MetadataEditorView, { global: { stubs, directives: { tooltip: () => {} } } })
}

// Drives the view the way the user does: open the folder dialog, pick the
// library, pick a folder. Identify does nothing until a library is selected, so
// every test needs this first.
async function openFolder(w: ReturnType<typeof mountView>) {
    await w.findAll('button')[0].trigger('click')
    w.findComponent({ name: 'Dropdown' }).vm.$emit('update:modelValue', 1)
    await flushPromises()
    w.findComponent({ name: 'FolderTree' }).vm.$emit('select', 'Artist/Album')
    await flushPromises()
}

beforeEach(() => {
    useIdentifyCache().clear()
    identifySpy.mockReset()
    identifyAlbumSpy.mockReset()
    refetchSpy.mockReset()
    tracksRef.value = tracks
})

describe('MetadataEditorView identify cache', () => {
    it('does not re-request when the track dialog is reopened for the same files', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const w = mountView()
        await openFolder(w)
        const panel = w.findComponent({ name: 'EditPanel' })

        panel.vm.$emit('identify', [mkTrack('a.mp3')])
        await flushPromises()
        expect(identifySpy).toHaveBeenCalledTimes(1)

        // Close the dialog the way Cancel does, then identify the same file again.
        w.findComponent({ name: 'IdentifyReviewDialog' }).vm.$emit('update:visible', false)
        await flushPromises()
        panel.vm.$emit('identify', [mkTrack('a.mp3')])
        await flushPromises()

        expect(identifySpy).toHaveBeenCalledTimes(1)
        const dialog = w.findComponent({ name: 'IdentifyReviewDialog' })
        expect(dialog.props('visible')).toBe(true)
        expect((dialog.props('results') as IdentifyTrackResult[]).map((r) => r.path)).toEqual([
            'a.mp3'
        ])
    })

    it('does not re-request when the album dialog is reopened for the same selection', async () => {
        identifyAlbumSpy.mockResolvedValue(albumResponse)
        const w = mountView()
        await openFolder(w)
        const panel = w.findComponent({ name: 'EditPanel' })

        panel.vm.$emit('identify-album', tracks)
        await flushPromises()
        expect(identifyAlbumSpy).toHaveBeenCalledTimes(1)

        w.findComponent({ name: 'IdentifyAlbumDialog' }).vm.$emit('update:visible', false)
        await flushPromises()
        panel.vm.$emit('identify-album', tracks)
        await flushPromises()

        expect(identifyAlbumSpy).toHaveBeenCalledTimes(1)
        const dialog = w.findComponent({ name: 'IdentifyAlbumDialog' })
        expect(dialog.props('visible')).toBe(true)
        expect(dialog.props('options')).toHaveLength(1)
    })

    it('re-requests after the dialog asks for a re-identify', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const w = mountView()
        await openFolder(w)
        w.findComponent({ name: 'EditPanel' }).vm.$emit('identify', [mkTrack('a.mp3')])
        await flushPromises()

        w.findComponent({ name: 'IdentifyReviewDialog' }).vm.$emit('reidentify')
        await flushPromises()

        expect(identifySpy).toHaveBeenCalledTimes(2)
    })

    it('re-requests after the album dialog asks for a re-identify', async () => {
        identifyAlbumSpy.mockResolvedValue(albumResponse)
        const w = mountView()
        await openFolder(w)
        w.findComponent({ name: 'EditPanel' }).vm.$emit('identify-album', tracks)
        await flushPromises()

        w.findComponent({ name: 'IdentifyAlbumDialog' }).vm.$emit('reidentify')
        await flushPromises()

        expect(identifyAlbumSpy).toHaveBeenCalledTimes(2)
    })

    it('forgets the cached answers when the folder is reloaded', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const w = mountView()
        await openFolder(w)
        const panel = w.findComponent({ name: 'EditPanel' })
        panel.vm.$emit('identify', [mkTrack('a.mp3')])
        await flushPromises()

        await w.find('[aria-label="Reload"]').trigger('click')
        await flushPromises()
        expect(refetchSpy).toHaveBeenCalledTimes(1)

        panel.vm.$emit('identify', [mkTrack('a.mp3')])
        await flushPromises()
        expect(identifySpy).toHaveBeenCalledTimes(2)
    })
})
