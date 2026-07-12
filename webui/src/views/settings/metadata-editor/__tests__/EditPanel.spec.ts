import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import EditPanel from '@/views/settings/metadata-editor/EditPanel.vue'
import type { Track } from '@/types/metadata'

vi.mock('@/components/library/MusicBrainzArtistPicker.vue', () => ({
    default: {
        name: 'MusicBrainzArtistPicker',
        props: ['visible', 'artistName', 'currentMbid'],
        emits: ['select', 'update:visible'],
        template: '<div />'
    }
}))

vi.mock('@/components/library/MusicBrainzAlbumPicker.vue', () => ({
    default: {
        name: 'MusicBrainzAlbumPicker',
        props: ['visible', 'albumName', 'currentReleaseMbid', 'currentReleaseGroupMbid'],
        emits: ['select', 'update:visible'],
        template: '<div />'
    }
}))

vi.mock('@/components/library/AlbumCoverPicker.vue', () => ({
    default: {
        name: 'AlbumCoverPicker',
        props: ['visible', 'albumName', 'releaseMbid', 'releaseGroupMbid', 'libraryId', 'paths'],
        emits: ['select', 'update:visible'],
        template: '<div class="album-cover-picker-stub" />'
    }
}))

// EditPanel calls useApplyCover()/useDeleteCover() at setup; keep the real pure
// helpers but stub the mutations so no vue-query/toast providers are needed.
const applyCoverSpy = vi.hoisted(() => vi.fn())
const deleteCoverSpy = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useApplyCover: () => ({
            mutateAsync: applyCoverSpy,
            isPending: { __v_isRef: true, value: false }
        }),
        useDeleteCover: () => ({
            mutateAsync: deleteCoverSpy,
            isPending: { __v_isRef: true, value: false }
        })
    }
})

// Keep getCurrentCoverUrl real (pure), stub the network calls.
const getCoverInfoSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', async (importActual) => {
    const actual = await importActual<typeof import('@/lib/api/Metadata')>()
    return {
        ...actual,
        getCoverInfo: (...args: unknown[]) => getCoverInfoSpy(...args),
        deleteCover: vi.fn()
    }
})

const stubs = {
    InputText: {
        props: ['modelValue', 'disabled'],
        template:
            '<input :disabled="disabled" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    InputNumber: {
        props: ['modelValue'],
        template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', Number($event.target.value))" />'
    },
    Checkbox: { props: ['modelValue'], template: '<input type="checkbox" />' },
    // inheritAttrs:false stops the parent's @click from ALSO falling through as a
    // native listener (which would fire the handler twice); aria-label is bound
    // explicitly so [aria-label=...] queries still resolve.
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :aria-label="$attrs[\'aria-label\']" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: '',
    artists: ['X'],
    album_artists: [],
    album: '',
    year: 0,
    disc_number: 0,
    disc_subtitle: '',
    compilation: false,
    mb_artist_ids: [''],
    mb_album_artist_ids: [],
    mb_release_id: '',
    mb_release_group_id: '',
    ...over
})

const clickSave = async (wrapper: any) => {
    const saveButton = wrapper.findAll('button').find((b: any) => b.text() === 'Save')
    await saveButton!.trigger('click')
    return wrapper.emitted('save')?.[0]?.[0] as any
}

describe('EditPanel album cover', () => {
    beforeEach(() => {
        getCoverInfoSpy.mockReset()
        getCoverInfoSpy.mockResolvedValue([])
        deleteCoverSpy.mockReset()
        deleteCoverSpy.mockResolvedValue(undefined)
    })

    it('lists every present cover source with its own thumbnail and remove button', async () => {
        getCoverInfoSpy.mockResolvedValue([
            { source: 'db' },
            { source: 'folder', detail: 'cover.jpg' }
        ])
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ path: 'album/t1.flac' })],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })
        await flushPromises()

        expect(wrapper.find('[data-test="cover-source-db"]').exists()).toBe(true)
        const folderRow = wrapper.find('[data-test="cover-source-folder"]')
        expect(folderRow.exists()).toBe(true)
        expect(folderRow.text()).toContain('album folder (cover.jpg)')
        expect(folderRow.find('img.cover-thumb').attributes('src')).toContain('source=folder')

        // Clicking Remove hides the row immediately but does NOT persist yet.
        await folderRow.find('button').trigger('click')
        expect(deleteCoverSpy).not.toHaveBeenCalled()
        expect(wrapper.find('[data-test="cover-source-folder"]').exists()).toBe(false)
        expect(wrapper.find('[data-test="cover-removals"]').exists()).toBe(true)

        // Save persists the staged removal.
        const saveButton = wrapper.findAll('button').find((b) => b.text() === 'Save')
        await saveButton!.trigger('click')
        expect(deleteCoverSpy).toHaveBeenCalledWith({
            libraryId: 3,
            path: 'album',
            source: 'folder',
            paths: undefined
        })
    })

    it('stages embedded removal and persists it (selected files only) on Save', async () => {
        getCoverInfoSpy.mockResolvedValue([{ source: 'embedded' }])
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ path: 'album/t1.flac' }), mkTrack({ path: 'album/t2.flac' })],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })
        await flushPromises()

        await wrapper.find('[data-test="cover-source-embedded"] button').trigger('click')
        expect(deleteCoverSpy).not.toHaveBeenCalled()

        const saveButton = wrapper.findAll('button').find((b) => b.text() === 'Save')
        await saveButton!.trigger('click')
        expect(deleteCoverSpy).toHaveBeenCalledWith({
            libraryId: 3,
            path: 'album',
            source: 'embedded',
            paths: ['album/t1.flac', 'album/t2.flac']
        })
    })

    it('undoes a staged removal before saving', async () => {
        getCoverInfoSpy.mockResolvedValue([{ source: 'folder', detail: 'cover.jpg' }])
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ path: 'album/t1.flac' })],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })
        await flushPromises()

        await wrapper.find('[data-test="cover-source-folder"] button').trigger('click')
        expect(wrapper.find('[data-test="cover-source-folder"]').exists()).toBe(false)

        await wrapper.find('[data-test="cover-removals"] button').trigger('click')
        expect(wrapper.find('[data-test="cover-source-folder"]').exists()).toBe(true)
        expect(wrapper.find('[data-test="cover-removals"]').exists()).toBe(false)
        expect(deleteCoverSpy).not.toHaveBeenCalled()
    })

    it('renders the album cover section with a change button', () => {
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ path: 'Beatles/Abbey Road/01.flac' })],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })
        expect(wrapper.text()).toContain('Album cover')
        const changeBtn = wrapper.findAll('button').find((b) => b.text() === 'Change cover…')
        expect(changeBtn).toBeTruthy()
    })

    it('opens the cover picker with the selected track paths', async () => {
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ path: 'album/t1.flac' }), mkTrack({ path: 'album/t2.flac' })],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })
        const changeBtn = wrapper.findAll('button').find((b) => b.text() === 'Change cover…')
        await changeBtn!.trigger('click')
        const picker = wrapper.findComponent({ name: 'AlbumCoverPicker' })
        expect(picker.props('visible')).toBe(true)
        expect(picker.props('paths')).toEqual(['album/t1.flac', 'album/t2.flac'])
        expect(picker.props('libraryId')).toBe(3)
    })

    it('requests cover info for the selected track’s own directory, not a parent', async () => {
        mount(EditPanel, {
            props: {
                // The tree selection can be a parent folder; the cover still lives in
                // the track's own directory, which is what must be queried.
                selection: [mkTrack({ path: 'Andrea Corr/01_Ten Feet High/08.mp3' })],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })
        await flushPromises()
        expect(getCoverInfoSpy).toHaveBeenCalledWith(3, 'Andrea Corr/01_Ten Feet High')
    })

    it('shows a note and skips cover info when the selection spans albums', async () => {
        const wrapper = mount(EditPanel, {
            props: {
                selection: [
                    mkTrack({ path: 'Album A/01.flac' }),
                    mkTrack({ path: 'Album B/01.flac' })
                ],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })
        await flushPromises()
        expect(getCoverInfoSpy).not.toHaveBeenCalled()
        expect(wrapper.find('[data-test="cover-multi-album"]').exists()).toBe(true)
        expect(wrapper.findAll('button').find((b) => b.text() === 'Change cover…')).toBeFalsy()
    })

    it('stages a chosen cover (no persist) and only writes it on Save', async () => {
        applyCoverSpy.mockReset()
        applyCoverSpy.mockResolvedValue({ ok: true, target: 'folder' })
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ path: 'album/t1.flac' })],
                isSaving: false,
                libraryId: 3
            },
            global: { stubs }
        })

        // Choosing a cover in the dialog only stages it — nothing is persisted yet.
        const picker = wrapper.findComponent({ name: 'AlbumCoverPicker' })
        picker.vm.$emit('select', { target: 'folder', file: null, imageUrl: 'http://img/x.jpg' })
        await wrapper.vm.$nextTick()
        expect(applyCoverSpy).not.toHaveBeenCalled()
        expect(wrapper.find('[data-test="cover-pending"]').exists()).toBe(true)

        // Save persists the staged cover via the cover endpoint.
        const saveButton = wrapper.findAll('button').find((b) => b.text() === 'Save')
        await saveButton!.trigger('click')
        expect(applyCoverSpy).toHaveBeenCalledTimes(1)
        const form = applyCoverSpy.mock.calls[0][0] as FormData
        expect(form.get('target')).toBe('folder')
        expect(form.get('image_url')).toBe('http://img/x.jpg')
        expect(form.getAll('paths')).toEqual(['album/t1.flac'])
    })
})

describe('EditPanel artist pairs', () => {
    it('renders one pair per artist with a blank ID input (no em dash)', () => {
        const wrapper = mount(EditPanel, {
            props: { selection: [mkTrack()], isSaving: false, libraryId: 1 },
            global: { stubs }
        })
        expect(wrapper.text()).not.toContain('—')
        expect(wrapper.findAll('input.pair-name')).toHaveLength(1)
        const mbid = wrapper.find('input.pair-mbid')
        expect((mbid.element as HTMLInputElement).value).toBe('')
    })

    it('sends only artist_mbids (not names) when just an ID is typed', async () => {
        const wrapper = mount(EditPanel, {
            props: { selection: [mkTrack()], isSaving: false, libraryId: 1 },
            global: { stubs }
        })
        await wrapper.find('input.pair-mbid').setValue('id-typed')
        const payload = await clickSave(wrapper)
        expect(payload.artist_mbids).toEqual({ X: 'id-typed' })
        expect(payload.artists).toBeUndefined()
    })

    it('adds an artist pair and writes the new name list plus its complete ID map', async () => {
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ artists: ['X'], mb_artist_ids: ['id-x'] })],
                isSaving: false,
                libraryId: 1
            },
            global: { stubs }
        })
        const addBtn = wrapper.findAll('button').find((b) => b.text() === 'Add artist')
        await addBtn!.trigger('click')
        const names = wrapper.findAll('input.pair-name')
        expect(names).toHaveLength(2)
        await names[1].setValue('New Artist')
        const payload = await clickSave(wrapper)
        expect(payload.artists).toEqual(['X', 'New Artist'])
        // complete map rebuilds the aligned tag; empty IDs travel as ''
        expect(payload.artist_mbids).toEqual({ X: 'id-x', 'New Artist': '' })
    })

    it('removes an artist pair and rewrites the name list', async () => {
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ artists: ['X', 'Y'], mb_artist_ids: ['id-x', 'id-y'] })],
                isSaving: false,
                libraryId: 1
            },
            global: { stubs }
        })
        const removeBtn = wrapper.findAll('button[aria-label="Remove artist"]')
        await removeBtn[1].trigger('click')
        const payload = await clickSave(wrapper)
        expect(payload.artists).toEqual(['X'])
        expect(payload.artist_mbids).toEqual({ X: 'id-x' })
    })

    it('fills both name and ID from the picker (flow b)', async () => {
        const wrapper = mount(EditPanel, {
            props: { selection: [mkTrack()], isSaving: false, libraryId: 1 },
            global: { stubs }
        })
        await wrapper.find('button[aria-label="Search MusicBrainz"]').trigger('click')
        const picker = wrapper.findComponent({ name: 'MusicBrainzArtistPicker' })
        picker.vm.$emit('select', 'id-b', 'The Beatles')
        await wrapper.vm.$nextTick()
        const payload = await clickSave(wrapper)
        expect(payload.artists).toEqual(['The Beatles'])
        expect(payload.artist_mbids).toEqual({ 'The Beatles': 'id-b' })
    })

    it('supports album-artist pairs symmetrically', async () => {
        const wrapper = mount(EditPanel, {
            props: {
                selection: [mkTrack({ album_artists: ['Y'], mb_album_artist_ids: [''] })],
                isSaving: false,
                libraryId: 1
            },
            global: { stubs }
        })
        // second pair-mbid input belongs to the album-artist block
        const mbidInputs = wrapper.findAll('input.pair-mbid')
        expect(mbidInputs).toHaveLength(2)
        await mbidInputs[1].setValue('id-y')
        const payload = await clickSave(wrapper)
        expect(payload.album_artist_mbids).toEqual({ Y: 'id-y' })
    })

    describe('album MusicBrainz IDs', () => {
        it('renders the release and release-group ID inputs', () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [mkTrack({ mb_release_id: 'rel-1', mb_release_group_id: 'rg-1' })],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            const ids = wrapper.findAll('input.album-mbid')
            expect(ids).toHaveLength(2)
            expect((ids[0].element as HTMLInputElement).value).toBe('rel-1')
            expect((ids[1].element as HTMLInputElement).value).toBe('rg-1')
        })

        it('sends only the release ID when it is typed manually', async () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [mkTrack()],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            await wrapper.findAll('input.album-mbid')[0].setValue('rel-typed')
            const payload = await clickSave(wrapper)
            expect(payload.mb_release_id).toBe('rel-typed')
            expect('mb_release_group_id' in payload).toBe(false)
            expect('album' in payload).toBe(false)
        })

        it('fills album name plus both IDs when a release is picked', async () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [mkTrack()],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            const picker = wrapper.findComponent({ name: 'MusicBrainzAlbumPicker' })
            picker.vm.$emit('select', 'rel-b', 'rg-b', 'OK Computer')
            await wrapper.vm.$nextTick()
            const payload = await clickSave(wrapper)
            expect(payload.album).toBe('OK Computer')
            expect(payload.mb_release_id).toBe('rel-b')
            expect(payload.mb_release_group_id).toBe('rg-b')
        })

        it('clears both IDs without touching the name when the match is cleared', async () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [
                        mkTrack({
                            album: 'Keep',
                            mb_release_id: 'rel-1',
                            mb_release_group_id: 'rg-1'
                        })
                    ],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            const picker = wrapper.findComponent({ name: 'MusicBrainzAlbumPicker' })
            picker.vm.$emit('select', '', '')
            await wrapper.vm.$nextTick()
            const payload = await clickSave(wrapper)
            expect(payload.mb_release_id).toBe('')
            expect(payload.mb_release_group_id).toBe('')
            expect('album' in payload).toBe(false)
        })
    })

    describe('disc fields', () => {
        it('renders disc number and disc subtitle inputs', () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [mkTrack({ disc_number: 2, disc_subtitle: 'CD 2' })],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            expect(wrapper.text()).toContain('Disc number')
            expect(wrapper.text()).toContain('Disc subtitle')
            expect(
                (wrapper.find('input.field-disc-number').element as HTMLInputElement).value
            ).toBe('2')
            expect(
                (wrapper.find('input.field-disc-subtitle').element as HTMLInputElement).value
            ).toBe('CD 2')
        })

        it('sends both disc fields when both are edited', async () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [mkTrack()],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            await wrapper.find('input.field-disc-number').setValue('2')
            await wrapper.find('input.field-disc-subtitle').setValue('Bonus Disc')
            const payload = await clickSave(wrapper)
            expect(payload.disc_number).toBe(2)
            expect(payload.disc_subtitle).toBe('Bonus Disc')
        })

        it('omits an untouched disc field from the patch', async () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [mkTrack()],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            // touch only disc number; disc subtitle must not appear in the patch
            await wrapper.find('input.field-disc-number').setValue('3')
            const payload = await clickSave(wrapper)
            expect(payload.disc_number).toBe(3)
            expect('disc_subtitle' in payload).toBe(false)
        })

        it('mass-edits disc number across a multi-track selection', async () => {
            const wrapper = mount(EditPanel, {
                props: {
                    selection: [
                        mkTrack({ path: 'a.mp3', disc_number: 1 }),
                        mkTrack({ path: 'b.mp3', disc_number: 1 })
                    ],
                    isSaving: false,
                    libraryId: 1
                },
                global: { stubs }
            })
            await wrapper.find('input.field-disc-number').setValue('2')
            const payload = await clickSave(wrapper)
            expect(payload.disc_number).toBe(2)
        })
    })

    describe('mixed multi-selection', () => {
        const selection = [
            mkTrack({ path: 'a.mp3', artists: ['A'], mb_artist_ids: [''] }),
            mkTrack({ path: 'b.mp3', artists: ['B'], mb_artist_ids: [''] })
        ]

        it('starts with a blank artist list and shows the mixed note', () => {
            const wrapper = mount(EditPanel, {
                props: { selection, isSaving: false, libraryId: 1 },
                global: { stubs }
            })
            // no artist rows to start; the whole list is overwritten by adding artists
            expect(wrapper.findAll('input.pair-name')).toHaveLength(0)
            expect(wrapper.text()).toContain('Selected tracks have different artists')
        })

        it('overwrites the whole list for every track when artists are added', async () => {
            const wrapper = mount(EditPanel, {
                props: { selection, isSaving: false, libraryId: 1 },
                global: { stubs }
            })
            const addBtn = wrapper.findAll('button').find((b) => b.text() === 'Add artist')
            await addBtn!.trigger('click')
            await wrapper.find('input.pair-name').setValue('New Artist')
            const payload = await clickSave(wrapper)
            expect(payload.artists).toEqual(['New Artist'])
            expect(payload.artist_mbids).toEqual({ 'New Artist': '' })
        })

        it('leaves artists untouched while saving other edited fields', async () => {
            const wrapper = mount(EditPanel, {
                props: { selection, isSaving: false, libraryId: 1 },
                global: { stubs }
            })
            // edit only the album; artists must not appear in the patch
            await wrapper.find('input.field-disc-subtitle').setValue('CD 2')
            const payload = await clickSave(wrapper)
            expect(payload.disc_subtitle).toBe('CD 2')
            expect('artists' in payload).toBe(false)
            expect('artist_mbids' in payload).toBe(false)
        })
    })
})
