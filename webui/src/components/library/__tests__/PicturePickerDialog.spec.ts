import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import PicturePickerDialog from '@/components/library/PicturePickerDialog.vue'
import type { CoverCandidate, PictureCopySource } from '@/types/metadata'
import type { MusicBrainzReleaseCandidate } from '@/types/artists'

const listReleaseCoversSpy = vi.hoisted(() => vi.fn())
const fetchPictureFileSpy = vi.hoisted(() => vi.fn())
const getPictureCandidateInfoSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', () => ({
    listReleaseCovers: (...args: unknown[]) => listReleaseCoversSpy(...args),
    fetchPictureFile: (...args: unknown[]) => fetchPictureFileSpy(...args),
    getPictureCandidateInfo: (...args: unknown[]) => getPictureCandidateInfoSpy(...args)
}))

const searchReleasesSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Artists', () => ({
    searchMusicBrainzReleases: (...args: unknown[]) => searchReleasesSpy(...args)
}))

const mkCandidate = (over: Partial<CoverCandidate> = {}): CoverCandidate => ({
    id: '1',
    thumbUrl: 'http://img/1-250.jpg',
    imageUrl: 'http://img/1.jpg',
    isFront: false,
    types: [],
    comment: '',
    ...over
})

const mkRelease = (over: Partial<MusicBrainzReleaseCandidate> = {}): MusicBrainzReleaseCandidate => ({
    releaseMbid: 'rel-m',
    releaseGroupMbid: 'rg-m',
    title: 'Manual Album',
    artist: 'Someone',
    artists: [],
    date: '1999',
    country: 'DE',
    trackCount: 10,
    disambiguation: '',
    score: 100,
    ...over
})

const mkSource = (over: Partial<PictureCopySource> = {}): PictureCopySource => ({
    key: 'Front Cover-embedded',
    label: 'Front cover — embedded in file',
    detail: '1 of 1 files',
    thumbUrl: 'http://api/pictures/image?slot=embedded',
    file: null,
    imageUrl: null,
    fetchUrl: 'http://api/pictures/image?slot=embedded',
    ...over
})

const stubs = {
    Dialog: {
        props: ['visible', 'header'],
        template:
            '<div v-if="visible"><div class="dialog-header">{{ header }}</div><slot /><slot name="footer" /></div>'
    },
    Button: {
        props: ['label', 'disabled', 'loading'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')">{{ label }}</button>'
    },
    InputText: {
        props: ['modelValue'],
        template:
            '<input :data-test="$attrs[\'data-test\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Message: { template: '<div class="message"><slot /></div>' }
}

function mountPicker(props: Record<string, unknown> = {}) {
    return mount(PicturePickerDialog, {
        props: {
            visible: true,
            pictureType: 'Back Cover',
            pictureSlot: 'folder',
            releaseMbid: 'rel-1',
            releaseGroupMbid: 'rg-1',
            ...props
        },
        // Tabs/TabList/Tab/TabPanels/TabPanel are real (they coordinate through
        // a provided context, which a shallow stub cannot reproduce).
        global: { stubs, plugins: [PrimeVue] }
    })
}

// openTab activates one of the dialog's tabs and waits for the panel swap.
async function openTab(wrapper: VueWrapper, name: 'copy' | 'search' | 'upload') {
    await wrapper.find(`[data-test="picture-tab-${name}"]`).trigger('click')
    await flushPromises()
}

// The search tab opens on the query step; most search assertions need the
// covers step, reached either through the MBID shortcut or a release row.
async function searchByMbid(wrapper: VueWrapper) {
    await openTab(wrapper, 'search')
    await wrapper.find('[data-test="picture-search"]').trigger('click')
    await flushPromises()
}

describe('PicturePickerDialog', () => {
    beforeEach(() => {
        listReleaseCoversSpy.mockReset()
        listReleaseCoversSpy.mockResolvedValue([])
        searchReleasesSpy.mockReset()
        searchReleasesSpy.mockResolvedValue([])
        fetchPictureFileSpy.mockReset()
        getPictureCandidateInfoSpy.mockReset()
        getPictureCandidateInfoSpy.mockResolvedValue({
            width: 1400,
            height: 1400,
            format: 'jpeg',
            bytes: 512000
        })
    })

    it('shows the type and slot in the header', () => {
        const wrapper = mountPicker({ pictureType: 'Back Cover', pictureSlot: 'folder' })
        expect(wrapper.find('.dialog-header').text()).toBe('Change back cover \u2014 album folder')
    })

    it('disables Select until a source is chosen', () => {
        const wrapper = mountPicker()
        expect(wrapper.find('[data-test="picture-select"]').attributes('disabled')).toBeDefined()
        expect(wrapper.find('[data-test="picture-chosen"]').text()).toBe('No image chosen yet')
    })

    describe('tabs', () => {
        it('opens on the copy tab when the album has images to copy', () => {
            const wrapper = mountPicker({ sources: [mkSource()] })
            expect(wrapper.find('[data-test="picture-tab-copy"]').text()).toContain(
                'This album (1)'
            )
            expect(wrapper.find('[data-test="picture-sources"]').exists()).toBe(true)
            expect(wrapper.find('[data-test="picture-manual-query"]').exists()).toBe(false)
        })

        it('opens on the search tab and hides the copy tab with no copy sources', () => {
            const wrapper = mountPicker()
            expect(wrapper.find('[data-test="picture-tab-copy"]').exists()).toBe(false)
            expect(wrapper.find('[data-test="picture-sources"]').exists()).toBe(false)
            expect(wrapper.find('[data-test="picture-manual-query"]').exists()).toBe(true)
        })

        it('switches between the three panels', async () => {
            const wrapper = mountPicker({ sources: [mkSource()] })
            await openTab(wrapper, 'search')
            expect(wrapper.find('[data-test="picture-manual-query"]').exists()).toBe(true)
            expect(wrapper.find('[data-test="picture-upload"]').exists()).toBe(false)

            await openTab(wrapper, 'upload')
            expect(wrapper.find('[data-test="picture-upload"]').exists()).toBe(true)
            expect(wrapper.find('[data-test="picture-manual-query"]').exists()).toBe(false)

            await openTab(wrapper, 'copy')
            expect(wrapper.find('[data-test="picture-sources"]').exists()).toBe(true)
        })

        it('keeps the chosen image visible in the footer across tabs', async () => {
            const wrapper = mountPicker({ sources: [mkSource()] })
            await wrapper.find('[data-test="picture-source-Front Cover-embedded"]').trigger('click')
            expect(wrapper.find('[data-test="picture-chosen"]').text()).toContain('Front cover')

            await openTab(wrapper, 'upload')
            // The panel changed, but the pending choice (and Select) survive.
            expect(wrapper.find('[data-test="picture-chosen"]').text()).toContain('Front cover')
            expect(
                wrapper.find('[data-test="picture-select"]').attributes('disabled')
            ).toBeUndefined()
        })
    })

    describe('search tab', () => {
        it('does NOT search on open; the MBID shortcut triggers it', async () => {
            const wrapper = mountPicker()
            await flushPromises()
            expect(listReleaseCoversSpy).not.toHaveBeenCalled()

            await searchByMbid(wrapper)
            expect(listReleaseCoversSpy).toHaveBeenCalledWith('rel-1', 'rg-1')
        })

        it('disables the MBID shortcut and says why without release MBIDs', async () => {
            const wrapper = mountPicker({ releaseMbid: '', releaseGroupMbid: '' })
            await openTab(wrapper, 'search')
            expect(
                wrapper.find('[data-test="picture-search"]').attributes('disabled')
            ).toBeDefined()
            expect(wrapper.find('.shortcut-note').text()).toContain('MusicBrainz ID')
        })

        it('searches releases by title, then shows the picked release\u2019s covers', async () => {
            searchReleasesSpy.mockResolvedValue([mkRelease()])
            listReleaseCoversSpy.mockResolvedValue([mkCandidate({ id: 'm' })])
            const wrapper = mountPicker({ albumName: 'Manual Album' })
            await openTab(wrapper, 'search')

            // The query prefills from the album name; no search runs on open.
            expect(
                (wrapper.find('[data-test="picture-manual-query"]').element as HTMLInputElement)
                    .value
            ).toBe('Manual Album')
            expect(searchReleasesSpy).not.toHaveBeenCalled()

            await wrapper.find('[data-test="picture-manual-search"]').trigger('click')
            await flushPromises()
            expect(searchReleasesSpy).toHaveBeenCalledWith('Manual Album')

            // Step 1 lists releases only \u2014 no cover tiles yet.
            expect(wrapper.findAll('.release-row')).toHaveLength(1)
            expect(wrapper.findAll('.cover-tile')).toHaveLength(0)

            await wrapper.find('.release-row').trigger('click')
            await flushPromises()
            // Step 2 swaps the release list for that release\u2019s covers.
            expect(listReleaseCoversSpy).toHaveBeenCalledWith('rel-m', 'rg-m')
            expect(wrapper.findAll('.release-row')).toHaveLength(0)
            expect(wrapper.find('[data-test="picture-release-note"]').text()).toContain(
                'Manual Album'
            )
            expect(wrapper.findAll('.cover-tile')).toHaveLength(1)
        })

        // The Cover Art Archive is flaky; a failed lookup has to read as a
        // sentence with a way to try again, not as a JSON dump.
        it('shows the server sentence and a retry when the cover lookup fails', async () => {
            listReleaseCoversSpy.mockRejectedValue({
                response: {
                    status: 502,
                    data: {
                        error: 'Cover Art Archive is temporarily unavailable. Try again in a few minutes.',
                        code: 'upstream_error'
                    }
                }
            })
            const wrapper = mountPicker()
            await searchByMbid(wrapper)

            const msg = wrapper.find('[data-test="picture-search-error"]')
            expect(msg.exists()).toBe(true)
            expect(msg.text()).toContain('Cover Art Archive is temporarily unavailable')
            expect(msg.text()).not.toContain('{')
            // "No images found" must not also claim the release simply has none.
            expect(wrapper.text()).not.toContain('No images found')

            listReleaseCoversSpy.mockResolvedValue([mkCandidate({ id: 'retried' })])
            await wrapper.find('[data-test="picture-search-retry"]').trigger('click')
            await flushPromises()
            expect(wrapper.find('[data-test="picture-search-error"]').exists()).toBe(false)
            expect(wrapper.findAll('.cover-tile')).toHaveLength(1)
        })

        it('retries the picked release, not the album MBID, after a failure', async () => {
            searchReleasesSpy.mockResolvedValue([mkRelease()])
            listReleaseCoversSpy.mockRejectedValue({
                response: { status: 502, data: { error: 'upstream is down', code: 'upstream_error' } }
            })
            const wrapper = mountPicker({ albumName: 'Manual Album' })
            await openTab(wrapper, 'search')
            await wrapper.find('[data-test="picture-manual-search"]').trigger('click')
            await flushPromises()
            await wrapper.find('.release-row').trigger('click')
            await flushPromises()
            expect(wrapper.find('[data-test="picture-search-error"]').exists()).toBe(true)

            listReleaseCoversSpy.mockClear()
            listReleaseCoversSpy.mockResolvedValue([])
            await wrapper.find('[data-test="picture-search-retry"]').trigger('click')
            await flushPromises()
            expect(listReleaseCoversSpy).toHaveBeenCalledWith('rel-m', 'rg-m')
        })

        it('shows the release-search failure as a sentence too', async () => {
            searchReleasesSpy.mockRejectedValue({
                response: {
                    status: 429,
                    data: {
                        error: 'MusicBrainz is receiving too many requests right now. Wait a moment and try again.',
                        code: 'upstream_rate_limited'
                    }
                }
            })
            const wrapper = mountPicker({ albumName: 'Manual Album' })
            await openTab(wrapper, 'search')
            await wrapper.find('[data-test="picture-manual-search"]').trigger('click')
            await flushPromises()

            const msg = wrapper.find('[data-test="picture-release-error"]')
            expect(msg.exists()).toBe(true)
            expect(msg.text()).toContain('too many requests')
            expect(msg.text()).not.toContain('{')
            expect(wrapper.text()).not.toContain('No releases matched')
        })

        it('reports a failed copy download as a readable sentence', async () => {
            fetchPictureFileSpy.mockRejectedValue({
                response: {
                    status: 502,
                    data: { error: 'The image could not be downloaded.', code: 'upstream_error' }
                }
            })
            const wrapper = mountPicker({ sources: [mkSource()] })
            await wrapper.find('[data-test="picture-source-Front Cover-embedded"]').trigger('click')
            await wrapper.find('[data-test="picture-select"]').trigger('click')
            await flushPromises()

            const text = wrapper.find('[data-test="picture-sources"]').text()
            expect(text).toContain('The image could not be downloaded.')
            expect(text).not.toContain('{')
            // The dialog stays open so the user can pick something else.
            expect(wrapper.emitted('update:visible')).toBeUndefined()
        })

        it('goes back from the covers step to the release search', async () => {
            searchReleasesSpy.mockResolvedValue([mkRelease()])
            listReleaseCoversSpy.mockResolvedValue([mkCandidate()])
            const wrapper = mountPicker({ albumName: 'Manual Album' })
            await searchByMbid(wrapper)
            expect(wrapper.findAll('.cover-tile')).toHaveLength(1)

            await wrapper.find('[data-test="picture-covers-back"]').trigger('click')
            expect(wrapper.findAll('.cover-tile')).toHaveLength(0)
            expect(wrapper.find('[data-test="picture-manual-query"]').exists()).toBe(true)
        })

        it('works with no release MBIDs at all', async () => {
            searchReleasesSpy.mockResolvedValue([mkRelease()])
            const wrapper = mountPicker({
                releaseMbid: '',
                releaseGroupMbid: '',
                albumName: 'Untagged'
            })
            await openTab(wrapper, 'search')
            expect(
                wrapper.find('[data-test="picture-manual-search"]').attributes('disabled')
            ).toBeUndefined()
            await wrapper.find('[data-test="picture-manual-search"]').trigger('click')
            await flushPromises()
            expect(searchReleasesSpy).toHaveBeenCalledWith('Untagged')
        })

        it('prompts to search before one has run, even with a prefilled query', async () => {
            const wrapper = mountPicker({ albumName: 'Kid A' })
            await openTab(wrapper, 'search')
            // The prefilled title must not read as "searched and found nothing".
            expect(wrapper.find('.hint').text()).toContain('Search a release')

            searchReleasesSpy.mockResolvedValue([])
            await wrapper.find('[data-test="picture-manual-search"]').trigger('click')
            await flushPromises()
            expect(wrapper.find('.hint').text()).toContain('No releases matched')
        })

        it('disables the title search for a too-short query', async () => {
            const wrapper = mountPicker({ albumName: '' })
            await openTab(wrapper, 'search')
            expect(
                wrapper.find('[data-test="picture-manual-search"]').attributes('disabled')
            ).toBeDefined()
            await wrapper.find('[data-test="picture-manual-query"]').setValue('ab')
            expect(
                wrapper.find('[data-test="picture-manual-search"]').attributes('disabled')
            ).toBeUndefined()
        })

        it('sorts candidates matching the requested type first and tags them', async () => {
            listReleaseCoversSpy.mockResolvedValue([
                mkCandidate({ id: 'front', isFront: true, types: ['Front'] }),
                mkCandidate({ id: 'back', types: ['Back'] })
            ])
            const wrapper = mountPicker({ pictureType: 'Back Cover' })
            await searchByMbid(wrapper)

            const tiles = wrapper.findAll('.cover-tile')
            expect(tiles).toHaveLength(2)
            expect(tiles[0].text()).toContain('Back')
            expect(tiles[0].find('[data-test="type-match"]').exists()).toBe(true)
            expect(tiles[1].find('[data-test="type-match"]').exists()).toBe(false)
        })

        it('emits the selected candidate as {file: null, imageUrl} without persisting', async () => {
            listReleaseCoversSpy.mockResolvedValue([mkCandidate({ id: 'b', types: ['Back'] })])
            const wrapper = mountPicker()
            await searchByMbid(wrapper)

            await wrapper.find('.cover-tile').trigger('click')
            await wrapper.find('[data-test="picture-select"]').trigger('click')

            const emitted = wrapper.emitted('select')
            expect(emitted).toHaveLength(1)
            expect(emitted![0][0]).toEqual({
                file: null,
                imageUrl: 'http://img/1.jpg',
                previewUrl: 'http://img/1-250.jpg'
            })
            expect(wrapper.emitted('update:visible')![0]).toEqual([false])
        })

        it('probes a picked CAA candidate and shows its metadata', async () => {
            listReleaseCoversSpy.mockResolvedValue([
                mkCandidate({ id: 'b', imageUrl: 'http://img/b.jpg' })
            ])
            const wrapper = mountPicker()
            await searchByMbid(wrapper)
            await wrapper.find('.cover-tile').trigger('click')
            await flushPromises()

            expect(getPictureCandidateInfoSpy).toHaveBeenCalledWith('http://img/b.jpg')
            expect(wrapper.find('[data-test="candidate-meta"]').text()).toBe(
                '1400 × 1400 · JPEG · 500 KB'
            )
        })

        it('resets to the query step with no results when reopened', async () => {
            listReleaseCoversSpy.mockResolvedValue([mkCandidate()])
            const wrapper = mountPicker()
            await searchByMbid(wrapper)
            expect(wrapper.findAll('.cover-tile')).toHaveLength(1)

            await wrapper.setProps({ visible: false })
            await wrapper.setProps({ visible: true })
            await flushPromises()
            expect(wrapper.findAll('.cover-tile')).toHaveLength(0)
            expect(wrapper.find('[data-test="picture-manual-query"]').exists()).toBe(true)
            // ...and it still does not auto-search on the reopen.
            expect(listReleaseCoversSpy).toHaveBeenCalledTimes(1)
        })
    })

    describe('upload tab', () => {
        it('stages a chosen file and can clear it again', async () => {
            const url = URL as unknown as Record<string, unknown>
            url.createObjectURL = vi.fn(() => 'blob:upload')
            url.revokeObjectURL = vi.fn()
            const wrapper = mountPicker()
            await openTab(wrapper, 'upload')

            const file = new File(['x'], 'art.png', { type: 'image/png' })
            const input = wrapper.find('[data-test="picture-upload"]')
            Object.defineProperty(input.element, 'files', { value: [file] })
            await input.trigger('change')

            expect(wrapper.find('[data-test="picture-upload-preview"]').text()).toContain('art.png')
            expect(wrapper.find('[data-test="picture-chosen"]').text()).toContain('art.png')

            await wrapper.find('[data-test="picture-upload-clear"]').trigger('click')
            expect(wrapper.find('[data-test="picture-upload-preview"]').exists()).toBe(false)
            expect(wrapper.find('[data-test="picture-select"]').attributes('disabled')).toBeDefined()

            delete url.createObjectURL
            delete url.revokeObjectURL
        })
    })

    describe('copy sources', () => {
        it('lists the album\u2019s other images with their labels', () => {
            const wrapper = mountPicker({
                sources: [
                    mkSource(),
                    mkSource({
                        key: 'Back Cover-folder',
                        label: 'Back cover \u2014 album folder',
                        detail: 'back.jpg'
                    })
                ]
            })
            const tiles = wrapper.findAll('.source-tile')
            expect(tiles).toHaveLength(2)
            expect(tiles[0].text()).toContain('Front cover \u2014 embedded in file')
            expect(tiles[0].text()).toContain('1 of 1 files')
            expect(tiles[0].find('img').attributes('src')).toBe(
                'http://api/pictures/image?slot=embedded'
            )
        })

        it('downloads a server-held source and emits it as a staged file', async () => {
            const file = new File(['x'], 'copied.jpg', { type: 'image/jpeg' })
            fetchPictureFileSpy.mockResolvedValue(file)
            const wrapper = mountPicker({ sources: [mkSource()] })

            await wrapper.find('[data-test="picture-source-Front Cover-embedded"]').trigger('click')
            await wrapper.find('[data-test="picture-select"]').trigger('click')
            await flushPromises()

            expect(fetchPictureFileSpy).toHaveBeenCalledWith(
                'http://api/pictures/image?slot=embedded'
            )
            expect(wrapper.emitted('select')![0][0]).toEqual({ file, imageUrl: null })
            expect(wrapper.emitted('update:visible')![0]).toEqual([false])
        })

        it('hands a source already staged in this session over without a download', async () => {
            const staged = new File(['y'], 'up.png', { type: 'image/png' })
            const wrapper = mountPicker({
                sources: [mkSource({ file: staged, fetchUrl: null, detail: 'pending change' })]
            })
            await wrapper.find('[data-test="picture-source-Front Cover-embedded"]').trigger('click')
            await wrapper.find('[data-test="picture-select"]').trigger('click')
            await flushPromises()
            expect(fetchPictureFileSpy).not.toHaveBeenCalled()
            expect(wrapper.emitted('select')![0][0]).toEqual({ file: staged, imageUrl: null })
        })

        it('reports a failed copy and keeps the dialog open', async () => {
            fetchPictureFileSpy.mockRejectedValue(
                new Error('could not load the image (status 404)')
            )
            const wrapper = mountPicker({ sources: [mkSource()] })
            await wrapper.find('[data-test="picture-source-Front Cover-embedded"]').trigger('click')
            await wrapper.find('[data-test="picture-select"]').trigger('click')
            await flushPromises()
            expect(wrapper.emitted('select')).toBeUndefined()
            expect(wrapper.emitted('update:visible')).toBeUndefined()
            expect(wrapper.find('[data-test="picture-sources"]').text()).toContain('status 404')
        })

        it('picking a CAA candidate clears a chosen copy source', async () => {
            listReleaseCoversSpy.mockResolvedValue([mkCandidate({ id: 'caa' })])
            const wrapper = mountPicker({ sources: [mkSource()] })
            await wrapper.find('[data-test="picture-source-Front Cover-embedded"]').trigger('click')
            await searchByMbid(wrapper)
            await wrapper.find('.cover-tile').trigger('click')
            await wrapper.find('[data-test="picture-select"]').trigger('click')
            await flushPromises()
            expect(fetchPictureFileSpy).not.toHaveBeenCalled()
            expect(wrapper.emitted('select')![0][0]).toEqual({
                file: null,
                imageUrl: 'http://img/1.jpg',
                previewUrl: 'http://img/1-250.jpg'
            })
        })
    })
})
