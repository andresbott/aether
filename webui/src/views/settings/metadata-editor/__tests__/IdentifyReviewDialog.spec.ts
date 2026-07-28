import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import IdentifyReviewDialog from '@/views/settings/metadata-editor/IdentifyReviewDialog.vue'
import { ALL_IDENTIFY_FIELD_IDS, IDENTIFY_FIELDS } from '@/lib/identifyFields'
import type { IdentifyTrackResult, Track } from '@/types/metadata'

const stubs = {
    // Render dialog content inline so the body and footer are queryable.
    Dialog: {
        props: ['visible'],
        template: '<div v-if="visible"><slot /><slot name="footer" /></div>'
    },
    Dropdown: {
        props: ['modelValue', 'options', 'optionLabel', 'optionValue'],
        emits: ['update:modelValue'],
        inheritAttrs: false,
        template: `<select :data-test="$attrs['data-test']" :value="modelValue"
            @change="$emit('update:modelValue', options[$event.target.selectedIndex][optionValue])">
            <option v-for="o in options" :key="o[optionValue]" :value="o[optionValue]">{{ o[optionLabel] }}</option>
        </select>`
    },
    Checkbox: {
        props: ['modelValue', 'binary', 'inputId'],
        inheritAttrs: false,
        template:
            '<input type="checkbox" :id="inputId" :data-test="$attrs[\'data-test\']" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />'
    },
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: 'Current Title',
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

const highResult: IdentifyTrackResult = {
    path: 'a.mp3',
    candidates: [
        {
            score: 0.97,
            recording_mbid: 'rec-1',
            title: 'Matched Song',
            artists: [{ name: 'Artist', mbid: 'artist-1' }],
            releases: [
                { release_mbid: 'rel-1', release_group_mbid: 'rg-1', album: 'Album', year: 1999, track_number: 3, disc_number: 1 },
                { release_mbid: 'rel-2', release_group_mbid: 'rg-2', album: 'Best Of', year: 2005, track_number: 0, disc_number: 0 }
            ]
        },
        {
            score: 0.61,
            recording_mbid: 'rec-2',
            title: 'Weaker Match',
            artists: [],
            releases: []
        }
    ]
}

function mountDialog(
    results: IdentifyTrackResult[],
    tracks: Track[] = [mkTrack()],
    loading = false,
    pending: Track[] = tracks
) {
    return mount(IdentifyReviewDialog, {
        props: { visible: true, results, tracks, loading, pending },
        global: { stubs }
    })
}

describe('IdentifyReviewDialog loading state', () => {
    it('shows progress instead of rows while the request is in flight', () => {
        const pending = [mkTrack({ path: 'a.mp3' }), mkTrack({ path: 'b.mp3' })]
        const w = mountDialog([], pending, true, pending)
        expect(w.find('[data-test="identify-loading"]').exists()).toBe(true)
        expect(w.text()).toContain('Identifying 2 tracks')
        // Nothing to review yet, so no staging button to press.
        expect(w.find('[data-test="identify-apply"]').exists()).toBe(false)
    })

    it('counts a single pending track in the singular', () => {
        const pending = [mkTrack({ path: 'a.mp3' })]
        const w = mountDialog([], pending, true, pending)
        expect(w.text()).toContain('Identifying 1 track…')
    })

    it('swaps the progress state for the rows once results arrive', async () => {
        const w = mountDialog([], [mkTrack()], true)
        expect(w.find('[data-test="identify-loading"]').exists()).toBe(true)

        await w.setProps({ loading: false, results: [highResult] })
        expect(w.find('[data-test="identify-loading"]').exists()).toBe(false)
        expect(w.find('[data-test="identify-apply"]').exists()).toBe(true)
        expect(w.text()).toContain('Matched Song')
    })

    it('emits cancel alongside close so the parent can abort the request', async () => {
        const w = mountDialog([], [mkTrack()], true)
        await w.find('[data-test="identify-cancel"]').trigger('click')
        expect(w.emitted('cancel')).toHaveLength(1)
        expect(w.emitted('update:visible')![0]).toEqual([false])
    })

    it('emits cancel when the dialog is dismissed without the Cancel button', async () => {
        // The header X and Escape both come through the Dialog's update:visible,
        // and they must abort too — otherwise the request runs on invisibly.
        const w = mountDialog([], [mkTrack()], true)
        await w.findComponent(stubs.Dialog).vm.$emit('update:visible', false)
        expect(w.emitted('cancel')).toHaveLength(1)
    })

    it('still offers Cancel after results arrive', async () => {
        const w = mountDialog([highResult])
        await w.find('[data-test="identify-cancel"]').trigger('click')
        // No request is in flight by then; the parent's abort is a no-op.
        expect(w.emitted('update:visible')![0]).toEqual([false])
    })
})

describe('IdentifyReviewDialog', () => {
    it('pre-accepts confident matches and stages them on apply', async () => {
        const w = mountDialog([highResult])
        expect(w.find('[data-test="identify-title-a.mp3"]').text()).toBe('Matched Song')

        await w.find('[data-test="identify-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks).toHaveLength(1)
        expect(picks[0].path).toBe('a.mp3')
        expect(picks[0].candidate.recording_mbid).toBe('rec-1')
        // First release preselected.
        expect(picks[0].release.release_mbid).toBe('rel-1')
    })

    it('does not pre-accept a low-scoring match', async () => {
        const low: IdentifyTrackResult = {
            path: 'a.mp3',
            candidates: [{ ...highResult.candidates[1] }]
        }
        const w = mountDialog([low])
        const applyBtn = w.find('[data-test="identify-apply"]')
        expect(applyBtn.attributes('disabled')).toBeDefined()
    })

    it('unchecking a track excludes it from the picks', async () => {
        const w = mountDialog([highResult])
        await w.find('#accept-a\\.mp3').setValue(false)
        expect(w.find('[data-test="identify-apply"]').attributes('disabled')).toBeDefined()
    })

    it('choosing another candidate applies that candidate', async () => {
        const w = mountDialog([highResult])
        const candidates = w.find('[data-test="identify-candidate-a.mp3"]')
        expect(candidates.findAll('option')).toHaveLength(2)
        await candidates.setValue(1)
        await w.find('[data-test="identify-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks[0].candidate.recording_mbid).toBe('rec-2')
        expect(picks[0].release).toBeNull()
    })

    it('switching candidates restarts at the new candidate first release', async () => {
        // rec-1 has two releases; picking the second then switching to rec-2 (no
        // releases) must not carry index 1 over into an empty list.
        const w = mountDialog([highResult])
        await w.find('[data-test="identify-release-a.mp3"]').setValue(1)
        await w.find('[data-test="identify-candidate-a.mp3"]').setValue(1)
        await w.find('[data-test="identify-apply"]').trigger('click')
        expect((w.emitted('apply')![0][0] as any[])[0].release).toBeNull()
    })

    it('offers a release dropdown only when the candidate has several', async () => {
        const w = mountDialog([highResult])
        expect(w.find('[data-test="identify-release-a.mp3"]').exists()).toBe(true)

        // rec-2 carries no releases at all, so there is nothing to choose from.
        await w.find('[data-test="identify-candidate-a.mp3"]').setValue(1)
        expect(w.find('[data-test="identify-release-a.mp3"]').exists()).toBe(false)
        expect(w.find('[data-test="identify-release-none-a.mp3"]').exists()).toBe(true)
    })

    it('shows a single release as static text rather than a dropdown', () => {
        const single: IdentifyTrackResult = {
            path: 'a.mp3',
            candidates: [{ ...highResult.candidates[0], releases: [highResult.candidates[0].releases[0]] }]
        }
        const w = mountDialog([single])
        expect(w.find('[data-test="identify-release-a.mp3"]').exists()).toBe(false)
        expect(w.find('[data-test="identify-release-single-a.mp3"]').text()).toContain('Album (1999)')
    })

    it('shows per-track errors and no-match notes', () => {
        const w = mountDialog([
            { path: 'err.mp3', candidates: [], error: 'fingerprint failed' },
            { path: 'none.mp3', candidates: [] }
        ])
        expect(w.find('[data-test="identify-error"]').text()).toContain('fingerprint failed')
        expect(w.find('[data-test="identify-nomatch"]').exists()).toBe(true)
        expect(w.find('[data-test="identify-apply"]').attributes('disabled')).toBeDefined()
    })

    it('labels the apply button with the accepted count', () => {
        const w = mountDialog([highResult])
        expect(w.find('[data-test="identify-apply"]').text()).toBe('Stage 1 track')
    })
})

describe('IdentifyReviewDialog table', () => {
    it('shows the file name in its own column, with the full path only in a tooltip', () => {
        const w = mountDialog([{ ...highResult, path: 'CD 1/01 - song.mp3' }], [
            mkTrack({ path: 'CD 1/01 - song.mp3', name: '01 - song.mp3' })
        ])
        const cell = w.find('[data-test="identify-file-CD 1/01 - song.mp3"]')
        expect(cell.text()).toContain('01 - song.mp3')
        expect(cell.text()).not.toContain('CD 1/')
        expect(cell.find('.file-name').attributes('data-pd-tooltip')).toBeDefined()
    })

    it('marks a changed title and reports the current tag in its tooltip', () => {
        // With the title tag off the table, the tooltip is the only place it
        // appears — jsdom renders no tooltip text, so presence of the attribute is
        // what proves one was attached.
        const w = mountDialog([highResult])
        const title = w.find('[data-test="identify-title-a.mp3"]')
        expect(title.classes()).toContain('changed')
        expect(title.find('.cell-value').attributes('data-pd-tooltip')).toBeDefined()
    })

    it('does not mark a title the file already carries', () => {
        const w = mountDialog([highResult], [mkTrack({ title: 'Matched Song' })])
        const title = w.find('[data-test="identify-title-a.mp3"]')
        expect(title.classes()).not.toContain('changed')
        expect(title.find('.cell-value').attributes('data-pd-tooltip')).toBeUndefined()
    })

    it('shows unchanged when the match stages no artist', () => {
        const noArtist: IdentifyTrackResult = {
            path: 'a.mp3',
            candidates: [{ ...highResult.candidates[0], artists: [] }]
        }
        const w = mountDialog([noArtist])
        expect(w.find('[data-test="identify-artist-a.mp3"]').text()).toBe('unchanged')
    })

    it('marks a changed artist and reports the current tag in its tooltip', () => {
        // Same deal as the title: the file's own artist tag has no column, so the
        // target cell's tooltip is where it shows up.
        const w = mountDialog([highResult], [mkTrack({ artists: ['Old Artist'] })])
        const artist = w.find('[data-test="identify-artist-a.mp3"]')
        expect(artist.text()).toBe('Artist')
        expect(artist.classes()).toContain('changed')
        expect(artist.find('.cell-value').attributes('data-pd-tooltip')).toBeDefined()
    })

    it('does not mark an artist the file already carries', () => {
        const w = mountDialog([highResult], [mkTrack({ artists: ['Artist'] })])
        const artist = w.find('[data-test="identify-artist-a.mp3"]')
        expect(artist.classes()).not.toContain('changed')
        expect(artist.find('.cell-value').attributes('data-pd-tooltip')).toBeUndefined()
    })

    it('spends no column on the file current title or artist tags', () => {
        // Both live in the target cells' tooltips now; a column each only
        // repeated what the tooltip already says and squeezed the dropdowns.
        const w = mountDialog([highResult], [mkTrack({ artists: ['Old Artist'] })])
        const columns = w
            .find('.track-list-header')
            .findAll('span')
            .map((s) => s.text())
        expect(columns).toEqual(['', 'File', 'Title', 'Artist', 'Recording', 'Release'])
        const row = w.find('[data-test="identify-row-a.mp3"]').text()
        expect(row).not.toContain('Old Artist')
        expect(row).not.toContain('Current Title')
    })

    it('carries each candidate score in its recording option, so no separate match column is needed', () => {
        const w = mountDialog([highResult])
        const options = w.find('[data-test="identify-candidate-a.mp3"]').findAll('option')
        expect(options[0].text()).toContain('97%')
        expect(options[1].text()).toContain('61%')
    })

    it('renders an unmatched file as an inert row with no controls', () => {
        const w = mountDialog(
            [{ path: 'err.mp3', candidates: [], error: 'fingerprint failed' }],
            [mkTrack({ path: 'err.mp3', name: 'err.mp3' })]
        )
        const row = w.find('[data-test="identify-row-err.mp3"]')
        expect(row.classes()).toContain('unmatched')
        // Nothing to accept, choose, or re-point.
        expect(row.find('input[type="checkbox"]').exists()).toBe(false)
        expect(w.find('[data-test="identify-candidate-err.mp3"]').exists()).toBe(false)
        expect(w.find('[data-test="identify-release-none-err.mp3"]').exists()).toBe(false)
        expect(w.find('[data-test="identify-error"]').text()).toContain('fingerprint failed')
    })

    it('dims a row the user unaccepted without removing its controls', async () => {
        const w = mountDialog([highResult])
        await w.find('#accept-a\\.mp3').setValue(false)
        const row = w.find('[data-test="identify-row-a.mp3"]')
        expect(row.classes()).toContain('excluded')
        // Still re-pointable: the user may uncheck, adjust, then re-check.
        expect(w.find('[data-test="identify-candidate-a.mp3"]').exists()).toBe(true)
    })

    it('stripes alternating rows so a long batch stays scannable', () => {
        const w = mountDialog(
            [highResult, { ...highResult, path: 'b.mp3' }],
            [mkTrack(), mkTrack({ path: 'b.mp3', name: 'b.mp3' })]
        )
        expect(w.find('[data-test="identify-row-a.mp3"]').classes()).not.toContain('striped')
        expect(w.find('[data-test="identify-row-b.mp3"]').classes()).toContain('striped')
    })
})

describe('IdentifyReviewDialog field selection', () => {
    it('offers a checkbox per stageable field, all selected by default', async () => {
        const w = mountDialog([highResult])
        expect(w.find('[data-test="identify-fields"]').exists()).toBe(true)
        for (const field of IDENTIFY_FIELDS) {
            const box = w.find(`[data-test="identify-field-${field.id}"]`)
            expect(box.exists()).toBe(true)
            expect((box.element as HTMLInputElement).checked).toBe(true)
        }

        await w.find('[data-test="identify-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual([...ALL_IDENTIFY_FIELD_IDS])
    })

    it('hides the field row while the request is still in flight', () => {
        const w = mountDialog([], [mkTrack()], true)
        expect(w.find('[data-test="identify-fields"]').exists()).toBe(false)
    })

    it('puts All/None on the same row as the checkboxes they act on', () => {
        const w = mountDialog([highResult])
        const row = w.find('.fields-list')
        // Same parent element, not merely both present in the dialog.
        expect(row.find('[data-test="identify-fields-all"]').exists()).toBe(true)
        expect(row.find('[data-test="identify-fields-none"]').exists()).toBe(true)
        expect(row.find(`[data-test="identify-field-${IDENTIFY_FIELDS[0].id}"]`).exists()).toBe(true)
    })

    it('emits only the fields left checked', async () => {
        const w = mountDialog([highResult])
        await w.find('[data-test="identify-field-title"]').setValue(false)
        await w.find('[data-test="identify-field-artists"]').setValue(false)

        await w.find('[data-test="identify-apply"]').trigger('click')
        const fields = w.emitted('apply')![0][1] as string[]
        expect(fields).not.toContain('title')
        expect(fields).not.toContain('artists')
        expect(fields).toContain('album')
        // The picks themselves are unaffected: the narrowing is about which
        // values get staged, not which tracks were accepted.
        expect((w.emitted('apply')![0][0] as any[])[0].path).toBe('a.mp3')
    })

    it('None clears every field and blocks apply until something is picked', async () => {
        const w = mountDialog([highResult])
        await w.find('[data-test="identify-fields-none"]').trigger('click')
        expect(w.find('[data-test="identify-apply"]').attributes('disabled')).toBeDefined()

        await w.find('[data-test="identify-field-album"]').setValue(true)
        expect(w.find('[data-test="identify-apply"]').attributes('disabled')).toBeUndefined()
        await w.find('[data-test="identify-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual(['album'])
    })

    it('All restores the full selection after narrowing it', async () => {
        const w = mountDialog([highResult])
        await w.find('[data-test="identify-fields-none"]').trigger('click')
        await w.find('[data-test="identify-fields-all"]').trigger('click')
        await w.find('[data-test="identify-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual([...ALL_IDENTIFY_FIELD_IDS])
    })

    it('emits the fields in registry order regardless of click order', async () => {
        const w = mountDialog([highResult])
        await w.find('[data-test="identify-fields-none"]').trigger('click')
        await w.find('[data-test="identify-field-year"]').setValue(true)
        await w.find('[data-test="identify-field-title"]').setValue(true)
        await w.find('[data-test="identify-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual(['title', 'year'])
    })
})
