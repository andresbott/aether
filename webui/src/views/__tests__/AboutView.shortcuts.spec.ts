import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

vi.mock('@/composables/useVersion', () => ({
    useVersion: () => ({ data: ref(undefined) })
}))

import AboutView from '@/views/AboutView.vue'
import { VISIBLE_SHORTCUTS } from '@/utils/shortcuts'

const mountView = () => mount(AboutView)

describe('AboutView keyboard shortcuts section', () => {
    it('has a keyboard shortcuts section', () => {
        expect(mountView().text()).toContain('Keyboard shortcuts')
    })

    // Both this list and the help overlay read from SHORTCUTS, so they cannot
    // drift; this asserts the list actually renders the whole registry rather
    // than a hand-copied subset.
    it('lists every shortcut with its keys and its action', () => {
        const w = mountView()
        const rows = w.findAll('.shortcut-row')
        expect(rows).toHaveLength(VISIBLE_SHORTCUTS.length)

        const text = w.find('.shortcuts-table').text()
        for (const entry of VISIBLE_SHORTCUTS) {
            expect(text).toContain(entry.label)
            for (const key of entry.keys) expect(text).toContain(key)
        }
    })

    it('does not list the overlay-only Escape binding', () => {
        expect(mountView().find('.shortcuts-table').text()).not.toContain(
            'Close this overlay'
        )
    })

    it('tells the user which key opens the overlay', () => {
        // The whole point of the section is discoverability, so it has to name
        // the key that brings up the in-player help.
        const text = mountView().text()
        expect(text).toContain('?')
    })

    it('renders each key inside a kbd element so it reads as a key', () => {
        const w = mountView()
        expect(w.findAll('.shortcut-row kbd').length).toBeGreaterThanOrEqual(
            VISIBLE_SHORTCUTS.length
        )
    })
})
