import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'

import TrackSelectButton from '@/components/library/TrackSelectButton.vue'

describe('TrackSelectButton', () => {
    it('shows an empty circle when unselected and a check when selected', () => {
        expect(mount(TrackSelectButton).find('i').classes()).toContain('pi-circle')
        expect(
            mount(TrackSelectButton, { props: { selected: true } })
                .find('i')
                .classes()
        ).toContain('pi-check-circle')
    })

    it('reports its state to assistive tech', () => {
        const off = mount(TrackSelectButton)
        expect(off.attributes('aria-pressed')).toBe('false')
        expect(off.attributes('aria-label')).toBe('Select track')

        const on = mount(TrackSelectButton, { props: { selected: true } })
        expect(on.attributes('aria-pressed')).toBe('true')
        expect(on.attributes('aria-label')).toBe('Deselect track')
    })

    it('emits toggle on click', async () => {
        const w = mount(TrackSelectButton)
        await w.trigger('click')
        expect(w.emitted('toggle')).toHaveLength(1)
    })

    it('keeps the check visible while selected', () => {
        expect(mount(TrackSelectButton, { props: { selected: true } }).classes()).toContain(
            'is-selected'
        )
        expect(mount(TrackSelectButton).classes()).not.toContain('is-selected')
    })
})
