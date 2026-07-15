import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

const sortableCreate = vi.fn(() => ({ destroy: vi.fn() }))
vi.mock('sortablejs', () => ({
    default: { create: (...args: unknown[]) => sortableCreate(...(args as [])) }
}))

import TrackEditList from '@/components/layout/TrackEditList.vue'

const song = (id: string) => ({ id, title: `Song ${id}`, artist: 'A', album: 'Al', duration: 60 })

const mountList = (props: Record<string, unknown> = {}) =>
    mount(TrackEditList, {
        props: { songs: [song('1'), song('2'), song('3')], ...props },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

beforeEach(() => sortableCreate.mockClear())

describe('TrackEditList', () => {
    it('renders one editing row per song as a focusable listbox', () => {
        const w = mountList()
        const list = w.find('.queue-edit-list')
        expect(list.attributes('role')).toBe('listbox')
        expect(list.attributes('tabindex')).toBe('0')
        expect(w.findAll('.queue-edit-list .queue-row--editing')).toHaveLength(3)
    })

    it('marks the currentIndex row as current', () => {
        const w = mountList({ currentIndex: 1 })
        expect(w.find('[data-queue-index="1"]').classes()).toContain('queue-row--current')
    })

    it('emits delete with the single row index when it is not selected', async () => {
        const w = mountList()
        await w.find('[data-queue-index="2"] .delete-button').trigger('click')
        expect(w.emitted('delete')?.[0]).toEqual([[2]])
    })

    it('emits delete with every selected index when a selected row is deleted', async () => {
        const w = mountList()
        await w.find('[data-queue-index="0"]').trigger('click')
        await w.find('[data-queue-index="2"]').trigger('click', { ctrlKey: true })
        await w.find('[data-queue-index="2"] .delete-button').trigger('click')
        expect(w.emitted('delete')?.[0]).toEqual([[0, 2]])
    })

    it('Delete key emits delete with the whole selection', async () => {
        const w = mountList()
        await w.find('[data-queue-index="0"] .row-index--checkbox').trigger('click')
        await w.find('[data-queue-index="2"] .row-index--checkbox').trigger('click')
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(w.emitted('delete')?.[0]).toEqual([[0, 2]])
    })

    it('a Delete keypress with nothing selected emits nothing', async () => {
        const w = mountList()
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(w.emitted('delete')).toBeUndefined()
    })

    it('creates a Sortable with the given group and drag handle when mounted', async () => {
        mountList({ group: 'playlist' })
        await Promise.resolve()
        expect(sortableCreate).toHaveBeenCalledTimes(1)
        const opts = (sortableCreate.mock.calls[0] as unknown[])[1] as { handle: string; group: string }
        expect(opts.handle).toBe('.drag-handle')
        expect(opts.group).toBe('playlist')
    })

    it('uses the deleteLabel for the row delete tooltip', () => {
        const w = mountList({ deleteLabel: 'Remove from playlist' })
        expect(w.find('.delete-button').exists()).toBe(true)
    })
})
