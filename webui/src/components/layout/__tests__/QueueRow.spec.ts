import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

import QueueRow from '@/components/layout/QueueRow.vue'
import type { Song } from '@/types/subsonic'

const song: Song = { id: '1', title: 'Track', artist: 'Artist', duration: 60 }

const mountRow = (props: Record<string, unknown>) =>
    mount(QueueRow, {
        props: { song, queueIndex: 4, ...props },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

describe('QueueRow', () => {
    it('normal mode renders a play button with no remove/delete control', async () => {
        const w = mountRow({ editing: false })
        expect(w.find('button.queue-row').exists()).toBe(true)
        expect(w.find('.delete-button').exists()).toBe(false)
        expect(w.find('input[type="checkbox"]').exists()).toBe(false)
        expect(w.find('.track-number').text()).toBe('5') // queueIndex 4 → position 5
        await w.find('button.queue-row').trigger('click')
        expect(w.emitted('play')).toHaveLength(1)
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

    it('clicking the checkbox emits select via the row handler, not a separate action', async () => {
        const w = mountRow({ editing: true })
        await w.find('input[type="checkbox"]').trigger('click')
        expect(w.emitted('select')).toHaveLength(1)
        expect(w.emitted('select')![0]).toEqual([{ additive: false, range: false }])
    })

    it('ctrl-clicking the checkbox passes the additive modifier through the row handler', async () => {
        const w = mountRow({ editing: true })
        await w.find('input[type="checkbox"]').trigger('click', { ctrlKey: true })
        expect(w.emitted('select')![0]).toEqual([{ additive: true, range: false }])
    })

    it('the current row shows a play/pause toggle instead of a checkbox', () => {
        const w = mountRow({ editing: true, current: true })
        expect(w.find('input[type="checkbox"]').exists()).toBe(false)
        expect(w.find('.current-play-toggle').exists()).toBe(true)
    })

    it('the current row play toggle reflects the playing state', () => {
        const paused = mountRow({ editing: true, current: true, playing: false })
        expect(paused.find('.current-play-toggle .pi-play').exists()).toBe(true)
        const playing = mountRow({ editing: true, current: true, playing: true })
        expect(playing.find('.current-play-toggle .pi-pause').exists()).toBe(true)
    })

    it('clicking the current row play toggle emits togglePlay without selecting', async () => {
        const w = mountRow({ editing: true, current: true })
        await w.find('.current-play-toggle').trigger('click')
        expect(w.emitted('togglePlay')).toHaveLength(1)
        expect(w.emitted('select')).toBeUndefined()
    })

    it('the current row body click does not select it', async () => {
        const w = mountRow({ editing: true, current: true })
        await w.find('[role="option"]').trigger('click')
        expect(w.emitted('select')).toBeUndefined()
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

    it('exposes the queue index on the normal-mode row for drop targeting', () => {
        const w = mountRow({ editing: false })
        expect(w.find('button.queue-row').attributes('data-queue-index')).toBe('4')
    })
})
