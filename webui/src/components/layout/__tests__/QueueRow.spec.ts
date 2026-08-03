import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

// The view-mode row carries a favorite toggle, whose real mutation needs a
// vue-query client; `TrackFavoriteButton` has its own spec for the toggle itself.
const starMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: starMutate })
}))

import QueueRow from '@/components/layout/QueueRow.vue'
import type { Song } from '@/types/subsonic'

const song: Song = { id: '1', title: 'Track', artist: 'Artist', duration: 60 }

const mountRow = (props: Record<string, unknown>, over: Partial<Song> = {}) =>
    mount(QueueRow, {
        props: { song: { ...song, ...over }, queueIndex: 4, ...props },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

// The view-mode row hosts the favorite toggle, so it cannot be a <button> (nested
// buttons are invalid HTML) — it is a role="button" div instead.
const viewRow = (w: ReturnType<typeof mountRow>) => w.find('div.queue-row[role="button"]')

describe('QueueRow', () => {
    it('normal mode renders a play affordance with no remove/delete control', async () => {
        const w = mountRow({ editing: false })
        expect(viewRow(w).exists()).toBe(true)
        expect(w.find('button.queue-row').exists()).toBe(false)
        expect(w.find('.delete-button').exists()).toBe(false)
        expect(w.find('input[type="checkbox"]').exists()).toBe(false)
        expect(w.find('.track-number').text()).toBe('5') // queueIndex 4 → position 5
        await viewRow(w).trigger('click')
        expect(w.emitted('play')).toHaveLength(1)
    })

    // The div replaced a real <button>, which activated on Enter/Space for free.
    it('normal mode is keyboard-activatable like the button it replaced', async () => {
        const w = mountRow({ editing: false })
        expect(viewRow(w).attributes('tabindex')).toBe('0')
        await viewRow(w).trigger('keydown', { key: 'Enter' })
        await viewRow(w).trigger('keydown', { key: ' ' })
        expect(w.emitted('play')).toHaveLength(2)
    })

    it('normal mode ignores other keys', async () => {
        const w = mountRow({ editing: false })
        await viewRow(w).trigger('keydown', { key: 'a' })
        await viewRow(w).trigger('keydown', { key: 'ArrowDown' })
        expect(w.emitted('play')).toBeUndefined()
    })

    it('edit mode renders a checkbox indicator, drag handle and delete, with no play affordance', () => {
        const w = mountRow({ editing: true })
        expect(w.find('[role="option"]').exists()).toBe(true)
        expect(w.find('input[type="checkbox"]').exists()).toBe(true)
        expect(w.find('.drag-handle').exists()).toBe(true)
        expect(w.find('.delete-button').exists()).toBe(true)
        expect(w.find('.play-hover-icon').exists()).toBe(false)
    })

    it('the edit-mode checkbox reflects the selected prop', () => {
        const off = mountRow({ editing: true, selected: false })
        expect((off.find('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
        const on = mountRow({ editing: true, selected: true })
        expect((on.find('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true)
    })

    it('clicking the checkbox cell toggles selection additively without a modifier key', async () => {
        const w = mountRow({ editing: true })
        await w.find('.row-index--checkbox').trigger('click')
        // Exactly one select — the row's own plain-select handler is suppressed.
        expect(w.emitted('select')).toHaveLength(1)
        expect(w.emitted('select')![0]).toEqual([{ additive: true, range: false }])
    })

    it('the checkbox cell stays additive regardless of modifier keys', async () => {
        const w = mountRow({ editing: true })
        await w.find('.row-index--checkbox').trigger('click', { shiftKey: true, ctrlKey: true })
        expect(w.emitted('select')![0]).toEqual([{ additive: true, range: false }])
    })

    it('the now-playing row shows a checkbox and no play toggle in edit mode', () => {
        const w = mountRow({ editing: true, current: true })
        expect(w.find('input[type="checkbox"]').exists()).toBe(true)
        expect(w.find('.current-play-toggle').exists()).toBe(false)
    })

    it('the now-playing row is selectable via its checkbox cell', async () => {
        const w = mountRow({ editing: true, current: true })
        await w.find('.row-index--checkbox').trigger('click')
        expect(w.emitted('select')![0]).toEqual([{ additive: true, range: false }])
    })

    it('the now-playing row body click selects it like any other row', async () => {
        const w = mountRow({ editing: true, current: true })
        await w.find('[role="option"]').trigger('click')
        expect(w.emitted('select')![0]).toEqual([{ additive: false, range: false }])
    })

    it('the now-playing row carries the current-row accent class', () => {
        const w = mountRow({ editing: true, current: true })
        expect(w.find('[role="option"]').classes()).toContain('queue-row--current')
    })

    it('the current row keeps the drag handle and delete for full parity', () => {
        const w = mountRow({ editing: true, current: true })
        expect(w.find('.drag-handle').exists()).toBe(true)
        expect(w.find('.delete-button').exists()).toBe(true)
    })

    it('plain row click in edit mode emits select with additive=false, range=false', async () => {
        const w = mountRow({ editing: true })
        await w.find('[role="option"]').trigger('click')
        expect(w.emitted('select')![0]).toEqual([{ additive: false, range: false }])
    })

    it('ctrl row click in edit mode emits select with additive=true', async () => {
        const w = mountRow({ editing: true })
        await w.find('[role="option"]').trigger('click', { ctrlKey: true })
        expect(w.emitted('select')![0]).toEqual([{ additive: true, range: false }])
    })

    it('shift row click in edit mode emits select with range=true', async () => {
        const w = mountRow({ editing: true })
        await w.find('[role="option"]').trigger('click', { shiftKey: true })
        expect(w.emitted('select')![0]).toEqual([{ additive: false, range: true }])
    })

    it('delete button emits delete without emitting select', async () => {
        const w = mountRow({ editing: true })
        await w.find('.delete-button').trigger('click')
        expect(w.emitted('delete')).toHaveLength(1)
        expect(w.emitted('select')).toBeUndefined()
    })

    it('reflects the selected prop via aria-selected', () => {
        const w = mountRow({ editing: true, selected: true })
        expect(w.find('[role="option"]').attributes('aria-selected')).toBe('true')
    })

    // useQueueDrop resolves drop positions from `[data-queue-index]` rects, so the
    // attribute has to survive the button→div change.
    it('exposes the queue index on the normal-mode row for drop targeting', () => {
        const w = mountRow({ editing: false })
        expect(viewRow(w).attributes('data-queue-index')).toBe('4')
    })

    it('stacks the artist under the title by default', () => {
        const w = mountRow({ editing: false })
        expect(w.find('.row-info').classes()).not.toContain('row-info--columns')
    })

    it('lays the artist out as a column beside the title with artistColumn, in both modes', () => {
        const view = mountRow({ editing: false, artistColumn: true })
        expect(view.find('.row-info').classes()).toContain('row-info--columns')
        const edit = mountRow({ editing: true, artistColumn: true })
        expect(edit.find('.row-info').classes()).toContain('row-info--columns')
    })
})

describe('QueueRow favorite toggle', () => {
    // Its own cell, not inside `.row-end` with the duration: two fixed-width
    // columns line up down the list, one flex group drifts with each duration's
    // text width.
    it('renders the shared favorite toggle in its own cell in view mode', () => {
        const w = mountRow({ editing: false })
        expect(w.find('.row-star-cell .row-star').exists()).toBe(true)
        expect(w.find('.row-end .row-star').exists()).toBe(false)
    })

    // The full Now Playing view is a real multi-column table, so the star column
    // gets wider gutters there than in the narrow sidebar.
    it('marks the row for the wider column layout with artistColumn', () => {
        expect(
            mountRow({ editing: false, artistColumn: true }).find('.queue-row').classes()
        ).toContain('queue-row--columns')
        expect(mountRow({ editing: false }).find('.queue-row').classes()).not.toContain(
            'queue-row--columns'
        )
    })

    // Edit mode is for reordering and removal; its row-end holds the drag handle
    // and delete instead.
    it('renders no favorite toggle in edit mode', () => {
        expect(mountRow({ editing: true }).find('.row-star').exists()).toBe(false)
    })

    it('toggles the song with its current state', async () => {
        const w = mountRow({ editing: false }, { starred: '2026-02-01T00:00:00Z' })
        await w.find('.row-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: '1', starred: true })
    })

    // The whole view-mode row is the play affordance, so the heart must swallow
    // its click or starring would also start playback.
    it('does not play the row when the heart is used', async () => {
        const w = mountRow({ editing: false })
        await w.find('.row-star').trigger('click')
        expect(w.emitted('play')).toBeUndefined()
    })
})
