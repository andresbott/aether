import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PicturesSection from '@/views/settings/metadata-editor/PicturesSection.vue'
import { useEditSession } from '@/composables/useEditSession'
import type { PictureInfo, Track } from '@/types/metadata'

// The session composable calls useApplyPicture()/useDeletePicture() plus the
// query client and toast at setup; stub the side-effecting pieces.
const applyPictureSpy = vi.hoisted(() => vi.fn())
const deletePictureSpy = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useApplyPicture: () => ({ mutateAsync: applyPictureSpy }),
        useDeletePicture: () => ({ mutateAsync: deletePictureSpy })
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

// Keep getPictureUrl real (pure), stub the network call.
const getPicturesSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', async (importActual) => {
    const actual = await importActual<typeof import('@/lib/api/Metadata')>()
    return {
        ...actual,
        getPictures: (...args: unknown[]) => getPicturesSpy(...args)
    }
})

vi.mock('@/components/library/PicturePickerDialog.vue', () => ({
    default: {
        name: 'PicturePickerDialog',
        props: [
            'visible',
            'pictureType',
            'pictureSlot',
            'releaseMbid',
            'releaseGroupMbid',
            'albumName',
            'sources'
        ],
        emits: ['select', 'update:visible'],
        template: '<div class="picture-picker-stub" />'
    }
}))

const stubs = {
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :aria-label="$attrs[\'aria-label\']" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\', $event)">{{ label }}</button>'
    },
    Menu: {
        name: 'Menu',
        props: ['model', 'popup'],
        methods: { toggle() {} },
        template:
            '<ul class="add-menu"><li v-for="item in model" :key="item.label" class="add-menu-item" @click="item.command()">{{ item.label }}</li></ul>'
    }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'album/a.mp3',
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

function mountSection(selection: Track[], libraryId: number | null = 3) {
    const session = useEditSession(
        () => selection,
        () => libraryId
    )
    const wrapper = mount(PicturesSection, {
        props: {
            selection,
            libraryId,
            session,
            releaseMbid: 'rel-1',
            releaseGroupMbid: 'rg-1',
            albumName: 'The Album'
        },
        global: { stubs }
    })
    return { wrapper, session }
}

describe('PicturesSection', () => {
    beforeEach(() => {
        getPicturesSpy.mockReset()
        getPicturesSpy.mockResolvedValue([])
        applyPictureSpy.mockReset()
        deletePictureSpy.mockReset()
    })

    it('requests the matrix for the selected track’s own directory with the selection paths', async () => {
        mountSection([mkTrack({ path: 'Artist/Album/01.flac' })])
        await flushPromises()
        expect(getPicturesSpy).toHaveBeenCalledWith(3, 'Artist/Album', ['Artist/Album/01.flac'])
    })

    it('renders one block per present type with its three slots', async () => {
        const pictures: PictureInfo[] = [
            {
                type: 'Front Cover',
                slots: [
                    { slot: 'embedded', detail: '1 of 2 files' },
                    { slot: 'folder', detail: 'cover.jpg' }
                ]
            },
            { type: 'Back Cover', slots: [{ slot: 'db' }] }
        ]
        getPicturesSpy.mockResolvedValue(pictures)
        const { wrapper } = mountSection([mkTrack()])
        await flushPromises()

        const front = wrapper.find('[data-test="picture-type-Front Cover"]')
        expect(front.exists()).toBe(true)
        expect(front.findAll('.picture-cell')).toHaveLength(3)
        expect(front.find('[data-test="picture-cell-Front Cover-embedded"]').text()).toContain(
            '1 of 2 files'
        )
        expect(front.find('[data-test="picture-cell-Front Cover-folder"]').text()).toContain(
            'cover.jpg'
        )
        // Occupied cells get a thumbnail from the image endpoint; empty ones don't.
        expect(
            front
                .find('[data-test="picture-cell-Front Cover-folder"] img.cell-thumb')
                .attributes('src')
        ).toContain('slot=folder')
        expect(
            front.find('[data-test="picture-cell-Front Cover-db"] img.cell-thumb').exists()
        ).toBe(false)
        // An occupied cell flips its image to the controls; an empty one has no
        // flip layer — its add button IS the placeholder.
        expect(
            front.find('[data-test="picture-cell-Front Cover-folder"] .cell-flip').exists()
        ).toBe(true)
        expect(front.find('[data-test="picture-cell-Front Cover-db"] .cell-flip').exists()).toBe(
            false
        )
        expect(
            front.find('[data-test="picture-cell-Front Cover-db"] .cell-art button').exists()
        ).toBe(true)

        expect(wrapper.find('[data-test="picture-type-Back Cover"]').exists()).toBe(true)
        // Absent types are not rendered.
        expect(wrapper.find('[data-test="picture-type-Media"]').exists()).toBe(false)
    })

    it('offers only absent types in the Add picture menu and adds an empty block', async () => {
        getPicturesSpy.mockResolvedValue([
            { type: 'Front Cover', slots: [{ slot: 'folder', detail: 'cover.jpg' }] }
        ])
        const { wrapper } = mountSection([mkTrack()])
        await flushPromises()

        const items = wrapper.findAll('.add-menu-item')
        const labels = items.map((i) => i.text())
        expect(labels).not.toContain('Front cover')
        expect(labels).toContain('Back cover')

        await items.find((i) => i.text() === 'Back cover')!.trigger('click')
        const block = wrapper.find('[data-test="picture-type-Back Cover"]')
        expect(block.exists()).toBe(true)
        // All-empty block: three cells, no thumbnails, no remove buttons.
        expect(block.findAll('.picture-cell')).toHaveLength(3)
        expect(block.find('img.cell-thumb').exists()).toBe(false)
        expect(block.find('[data-test="picture-remove-Back Cover-folder"]').exists()).toBe(false)
    })

    it('stages a removal for an occupied cell and undoes it', async () => {
        getPicturesSpy.mockResolvedValue([
            { type: 'Back Cover', slots: [{ slot: 'folder', detail: 'back.jpg' }] }
        ])
        const { wrapper, session } = mountSection([mkTrack()])
        await flushPromises()

        await wrapper.find('[data-test="picture-remove-Back Cover-folder"]').trigger('click')
        expect(deletePictureSpy).not.toHaveBeenCalled()
        expect(session.getPictureOp('album', 'Back Cover', 'folder')).toEqual({
            kind: 'remove',
            paths: ['album/a.mp3']
        })
        const cell = wrapper.find('[data-test="picture-cell-Back Cover-folder"]')
        expect(cell.classes()).toContain('removing')

        await wrapper.find('[data-test="picture-undo-Back Cover-folder"]').trigger('click')
        expect(session.getPictureOp('album', 'Back Cover', 'folder')).toBeUndefined()
        expect(session.hasStagedChanges.value).toBe(false)
    })

    it('opens the picker scoped to the clicked cell and stages its selection', async () => {
        getPicturesSpy.mockResolvedValue([
            { type: 'Back Cover', slots: [{ slot: 'folder', detail: 'back.jpg' }] }
        ])
        const { wrapper, session } = mountSection([mkTrack()])
        await flushPromises()

        await wrapper.find('[data-test="picture-change-Back Cover-embedded"]').trigger('click')
        const picker = wrapper.findComponent({ name: 'PicturePickerDialog' })
        expect(picker.props('visible')).toBe(true)
        expect(picker.props('pictureType')).toBe('Back Cover')
        expect(picker.props('pictureSlot')).toBe('embedded')
        expect(picker.props('releaseMbid')).toBe('rel-1')

        picker.vm.$emit('select', { file: null, imageUrl: 'http://img/x.jpg' })
        await wrapper.vm.$nextTick()
        expect(applyPictureSpy).not.toHaveBeenCalled()
        const op = session.getPictureOp('album', 'Back Cover', 'embedded')
        expect(op?.kind).toBe('set')
        expect(
            wrapper.find('[data-test="picture-cell-Back Cover-embedded"]').classes()
        ).toContain('pending')
    })

    describe('picker copy sources', () => {
        it('offers the album’s other occupied cells, excluding the target cell', async () => {
            getPicturesSpy.mockResolvedValue([
                {
                    type: 'Front Cover',
                    slots: [
                        { slot: 'embedded', detail: '1 of 1 files' },
                        { slot: 'folder', detail: 'cover.jpg' }
                    ]
                },
                { type: 'Back Cover', slots: [{ slot: 'db' }] }
            ])
            const { wrapper } = mountSection([mkTrack()])
            await flushPromises()

            // Editing the (empty) internal-store front cover: every other
            // occupied cell of the album is a copy source.
            await wrapper.find('[data-test="picture-change-Front Cover-db"]').trigger('click')
            const sources = wrapper
                .findComponent({ name: 'PicturePickerDialog' })
                .props('sources') as Array<Record<string, unknown>>
            expect(sources.map((s) => s.key)).toEqual([
                'Front Cover-embedded',
                'Front Cover-folder',
                'Back Cover-db'
            ])
            const embedded = sources[0]
            expect(embedded.label).toBe('Front cover — embedded in file')
            expect(embedded.detail).toBe('1 of 1 files')
            // Server-held images are downloaded by the picker from the image endpoint.
            expect(embedded.file).toBeNull()
            expect(String(embedded.fetchUrl)).toContain('slot=embedded')
        })

        it('excludes the edited cell itself and empty cells', async () => {
            getPicturesSpy.mockResolvedValue([
                { type: 'Front Cover', slots: [{ slot: 'folder', detail: 'cover.jpg' }] }
            ])
            const { wrapper } = mountSection([mkTrack()])
            await flushPromises()

            await wrapper.find('[data-test="picture-change-Front Cover-folder"]').trigger('click')
            expect(
                wrapper.findComponent({ name: 'PicturePickerDialog' }).props('sources')
            ).toEqual([])
        })

        it('offers an image staged this session, carrying its file rather than a URL', async () => {
            // jsdom does not implement object URLs; the session previews
            // uploads with one, so install a stub for this test.
            const url = URL as unknown as Record<string, unknown>
            url.createObjectURL = vi.fn(() => 'blob:preview')
            url.revokeObjectURL = vi.fn()
            const { wrapper, session } = mountSection([mkTrack()])
            await flushPromises()
            const file = new File(['x'], 'up.png', { type: 'image/png' })
            session.stagePictureSet('album', 'Front Cover', 'folder', { file, imageUrl: null }, [
                'album/a.mp3'
            ])
            await flushPromises()

            await wrapper.find('[data-test="picture-change-Front Cover-db"]').trigger('click')
            const sources = wrapper
                .findComponent({ name: 'PicturePickerDialog' })
                .props('sources') as Array<Record<string, unknown>>
            expect(sources).toHaveLength(1)
            expect(sources[0].key).toBe('Front Cover-folder')
            expect(sources[0].detail).toBe('pending change in this session')
            expect(sources[0].file).toBe(file)
            expect(sources[0].fetchUrl).toBeNull()
            expect(sources[0].thumbUrl).toBe('blob:preview')
            delete url.createObjectURL
            delete url.revokeObjectURL
        })

        it('does not offer a cell staged for removal', async () => {
            getPicturesSpy.mockResolvedValue([
                { type: 'Front Cover', slots: [{ slot: 'folder', detail: 'cover.jpg' }] }
            ])
            const { wrapper } = mountSection([mkTrack()])
            await flushPromises()
            await wrapper.find('[data-test="picture-remove-Front Cover-folder"]').trigger('click')

            await wrapper.find('[data-test="picture-change-Front Cover-db"]').trigger('click')
            expect(
                wrapper.findComponent({ name: 'PicturePickerDialog' }).props('sources')
            ).toEqual([])
        })

        it('passes the album name through for the manual release search', async () => {
            const { wrapper } = mountSection([mkTrack()])
            await flushPromises()
            expect(
                wrapper.findComponent({ name: 'PicturePickerDialog' }).props('albumName')
            ).toBe('The Album')
        })
    })

    it('hides an embedded op staged for another track in the same folder', async () => {
        const trackA = mkTrack({ path: 'album/01.flac' })
        const trackB = mkTrack({ path: 'album/02.flac' })
        const { wrapper, session } = mountSection([trackA, trackB])
        await flushPromises()

        // Stage an embedded back cover for track A only, then select track B.
        session.stagePictureSet(
            'album',
            'Back Cover',
            'embedded',
            { file: null, imageUrl: 'http://img/x.jpg' },
            ['album/01.flac']
        )
        await wrapper.setProps({ selection: [trackB] })
        await flushPromises()

        // Same folder, but the op does not concern track B: no block, no
        // pending cell, no thumbnail.
        expect(wrapper.find('[data-test="picture-type-Back Cover"]').exists()).toBe(false)

        // Back on track A the staged op is visible again.
        await wrapper.setProps({ selection: [trackA] })
        await flushPromises()
        const cell = wrapper.find('[data-test="picture-cell-Back Cover-embedded"]')
        expect(cell.exists()).toBe(true)
        expect(cell.classes()).toContain('pending')
    })

    it('still shows folder/db ops staged from another track of the album', async () => {
        const trackA = mkTrack({ path: 'album/01.flac' })
        const trackB = mkTrack({ path: 'album/02.flac' })
        const { wrapper, session } = mountSection([trackA, trackB])
        await flushPromises()

        // Folder art belongs to the whole album, whoever staged it.
        session.stagePictureSet(
            'album',
            'Back Cover',
            'folder',
            { file: null, imageUrl: 'http://img/x.jpg' },
            ['album/01.flac']
        )
        await wrapper.setProps({ selection: [trackB] })
        await flushPromises()
        const cell = wrapper.find('[data-test="picture-cell-Back Cover-folder"]')
        expect(cell.exists()).toBe(true)
        expect(cell.classes()).toContain('pending')
    })

    it('refetches when the selection moves to another track in the same folder', async () => {
        // Track 1 carries an embedded back cover; track 2 has nothing.
        getPicturesSpy.mockResolvedValue([
            { type: 'Back Cover', slots: [{ slot: 'embedded', detail: '1 of 1 files' }] }
        ])
        const { wrapper } = mountSection([mkTrack({ path: 'album/01.flac' })])
        await flushPromises()
        expect(wrapper.find('[data-test="picture-type-Back Cover"]').exists()).toBe(true)

        getPicturesSpy.mockResolvedValue([])
        await wrapper.setProps({ selection: [mkTrack({ path: 'album/02.flac' })] })
        await flushPromises()
        // Same directory, different track: the matrix must be refetched with
        // the new paths and the stale type block must disappear.
        expect(getPicturesSpy).toHaveBeenLastCalledWith(3, 'album', ['album/02.flac'])
        expect(wrapper.find('[data-test="picture-type-Back Cover"]').exists()).toBe(false)
    })

    it('shows a note instead of the matrix when the selection spans albums', async () => {
        const { wrapper } = mountSection([
            mkTrack({ path: 'Album A/01.flac' }),
            mkTrack({ path: 'Album B/01.flac' })
        ])
        await flushPromises()
        expect(getPicturesSpy).not.toHaveBeenCalled()
        expect(wrapper.find('[data-test="pictures-multi-album"]').exists()).toBe(true)
        expect(wrapper.find('[data-test="add-picture"]').exists()).toBe(false)
    })

    it('shows the empty note when nothing is present or staged', async () => {
        const { wrapper } = mountSection([mkTrack()])
        await flushPromises()
        expect(wrapper.find('[data-test="no-pictures"]').exists()).toBe(true)
    })
})
