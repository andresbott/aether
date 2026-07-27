import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import IdentifyReviewDialog from '@/views/settings/metadata-editor/IdentifyReviewDialog.vue'
import type { IdentifyTrackResult, Track } from '@/types/metadata'

const stubs = {
    // Render dialog content inline so the body and footer are queryable.
    Dialog: {
        props: ['visible'],
        template: '<div v-if="visible"><slot /><slot name="footer" /></div>'
    },
    Dropdown: {
        props: ['modelValue', 'options'],
        template: '<select />'
    },
    Checkbox: {
        props: ['modelValue', 'binary', 'inputId'],
        template:
            '<input type="checkbox" :id="inputId" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />'
    },
    RadioButton: {
        props: ['modelValue', 'value', 'inputId'],
        template:
            '<input type="radio" :id="inputId" :checked="modelValue === value" @change="$emit(\'update:modelValue\', value)" />'
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
        expect(w.text()).toContain('Matched Song')
        expect(w.text()).toContain('current: Current Title')

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
        await w.find('input[type="checkbox"]').setValue(false)
        expect(w.find('[data-test="identify-apply"]').attributes('disabled')).toBeDefined()
    })

    it('choosing another candidate applies that candidate', async () => {
        const w = mountDialog([highResult])
        const radios = w.findAll('input[type="radio"]')
        expect(radios).toHaveLength(2)
        await radios[1].trigger('change')
        await w.find('[data-test="identify-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks[0].candidate.recording_mbid).toBe('rec-2')
        expect(picks[0].release).toBeNull()
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
