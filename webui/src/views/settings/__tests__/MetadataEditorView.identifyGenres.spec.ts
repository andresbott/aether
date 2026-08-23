import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import MetadataEditorView from '@/views/settings/MetadataEditorView.vue'
import { useIdentifyCache } from '@/composables/useIdentifyCache'
import type {
    AlbumIdentifyPick,
    AlbumOption,
    IdentifyPick,
    Track
} from '@/types/metadata'

// The identify dialogs resolve the genres themselves and hand them over on the
// picks; this spec is about the LAST hop — that the view forwards them into the
// staged overlay instead of dropping them on the floor. The dialogs are stubbed,
// so no genre lookup happens here.
const identifySpy = vi.hoisted(() => vi.fn())
const identifyAlbumSpy = vi.hoisted(() => vi.fn())
const tracksRef = vi.hoisted(() => ({ value: [] as Track[] }))

vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useTracks: () => ({ data: tracksRef, isLoading: { value: false }, refetch: vi.fn() }),
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

const tracks = [mkTrack('a.mp3'), mkTrack('b.mp3')]

const albumOption: AlbumOption = {
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

const trackPick: IdentifyPick = {
    path: 'a.mp3',
    candidate: {
        score: 0.97,
        recording_mbid: 'rec-1',
        title: 'Found',
        artists: [],
        releases: []
    },
    release: null,
    genres: ['Grunge', 'Alternative Rock']
}

const albumPick: AlbumIdentifyPick = {
    path: 'a.mp3',
    option: albumOption,
    assignment: null,
    genres: ['Grunge', 'Alternative Rock']
}

const stubs = {
    Dialog: {
        name: 'Dialog',
        props: ['visible'],
        template: '<div v-if="visible"><slot /><slot name="footer" /></div>'
    },
    ConfirmDialog: true,
    Select: {
        name: 'Select',
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
    // The edit session is passed down as `session`; the stub declares it so the
    // assertions can read the overlays the view staged.
    EditPanel: {
        name: 'EditPanel',
        props: ['selection', 'session'],
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

async function openFolder(w: ReturnType<typeof mountView>) {
    await w.findAll('button')[0].trigger('click')
    w.findComponent({ name: 'FolderTree' }).vm.$emit('select', 'Artist/Album')
    await flushPromises()
}

// The overlay the session staged for a path, read off the EditPanel's session.
function stagedOverlay(w: ReturnType<typeof mountView>, path: string) {
    const session = w.findComponent({ name: 'EditPanel' }).props('session') as {
        overlays: { value: Map<string, Record<string, unknown>> }
    }
    return session.overlays.value.get(path)
}

beforeEach(() => {
    useIdentifyCache().clear()
    identifySpy.mockReset()
    identifyAlbumSpy.mockReset()
    tracksRef.value = tracks
})

describe('MetadataEditorView identify genres', () => {
    it('stages the genres a track pick carries', async () => {
        const w = mountView()
        await openFolder(w)
        w.findComponent({ name: 'IdentifyReviewDialog' }).vm.$emit(
            'apply',
            [trackPick],
            ['genres']
        )
        await flushPromises()
        expect(stagedOverlay(w, 'a.mp3')?.genres).toEqual(['Grunge', 'Alternative Rock'])
    })

    it('stages the genres an album pick carries', async () => {
        const w = mountView()
        await openFolder(w)
        w.findComponent({ name: 'IdentifyAlbumDialog' }).vm.$emit('apply', [albumPick], ['genres'])
        await flushPromises()
        expect(stagedOverlay(w, 'a.mp3')?.genres).toEqual(['Grunge', 'Alternative Rock'])
    })

    it('stages no genres when the user unchecked the Genres field', async () => {
        // The pick still carries them; the field selection is what decides.
        const w = mountView()
        await openFolder(w)
        w.findComponent({ name: 'IdentifyReviewDialog' }).vm.$emit('apply', [trackPick], ['title'])
        await flushPromises()
        const overlay = stagedOverlay(w, 'a.mp3')
        expect(overlay?.genres).toBeUndefined()
        expect(overlay?.title).toBe('Found')
    })

    it('stages no genres when the pick resolved none', async () => {
        const w = mountView()
        await openFolder(w)
        w.findComponent({ name: 'IdentifyAlbumDialog' }).vm.$emit(
            'apply',
            [{ ...albumPick, genres: [] }],
            ['genres', 'album']
        )
        await flushPromises()
        const overlay = stagedOverlay(w, 'a.mp3')
        expect(overlay?.genres).toBeUndefined()
        expect(overlay?.album).toBe('Album A')
    })
})
