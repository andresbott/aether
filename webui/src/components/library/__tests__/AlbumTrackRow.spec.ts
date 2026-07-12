import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AlbumTrackRow from '@/components/library/AlbumTrackRow.vue'
import type { Song } from '@/types/subsonic'

const song = { id: 's1', title: 'Song One', artist: 'The Artist', duration: 125, track: 4 } as Song

const mountRow = (props: Partial<{ selected: boolean }> = {}) =>
    mount(AlbumTrackRow, {
        props: { song, index: 0, ...props },
        global: { directives: { tooltip: {} } }
    })

describe('AlbumTrackRow', () => {
    it('is draggable and shows track number, title, artist and duration columns', () => {
        const w = mountRow()
        expect(w.attributes('draggable')).toBe('true')
        expect(w.find('.track-number').text()).toBe('4')
        expect(w.find('.col-title').text()).toBe('Song One')
        expect(w.find('.col-artist').text()).toBe('The Artist')
        expect(w.find('.row-duration').text()).toBe('2:05')
    })

    it('does not render a hover play button', () => {
        expect(mountRow().find('.play-hover-icon').exists()).toBe(false)
    })

    it('applies the selected class when selected', () => {
        expect(mountRow({ selected: true }).classes()).toContain('selected')
        expect(mountRow().classes()).not.toContain('selected')
    })

    it('emits select with plain modifiers on a plain click', async () => {
        const w = mountRow()
        await w.trigger('click')
        expect(w.emitted('select')?.[0]).toEqual([{ additive: false, range: false }])
    })

    it('maps ctrl/meta to additive and shift to range', async () => {
        const w = mountRow()
        await w.trigger('click', { metaKey: true })
        await w.trigger('click', { shiftKey: true })
        expect(w.emitted('select')?.[0]).toEqual([{ additive: true, range: false }])
        expect(w.emitted('select')?.[1]).toEqual([{ additive: false, range: true }])
    })

    it('emits play on double-click', async () => {
        const w = mountRow()
        await w.trigger('dblclick')
        expect(w.emitted('play')).toHaveLength(1)
    })

    it('forwards dragstart and dragend', async () => {
        const w = mountRow()
        await w.trigger('dragstart')
        await w.trigger('dragend')
        expect(w.emitted('dragstart')).toHaveLength(1)
        expect(w.emitted('dragend')).toHaveLength(1)
    })
})
