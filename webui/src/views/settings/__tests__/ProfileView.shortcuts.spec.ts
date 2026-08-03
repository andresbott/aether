import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ProfileView from '@/views/settings/ProfileView.vue'
import { VISIBLE_SHORTCUTS } from '@/utils/shortcuts'

describe('ProfileView keyboard shortcuts section', () => {
    it('has a keyboard shortcuts section', () => {
        expect(mount(ProfileView).text()).toContain('Keyboard shortcuts')
    })

    // Both this list and the help overlay read from SHORTCUTS, so they cannot
    // drift; this asserts the list actually renders the whole registry rather
    // than a hand-copied subset.
    it('lists every shortcut with its keys and its action', () => {
        const w = mount(ProfileView)
        const rows = w.findAll('.shortcut-row')
        expect(rows).toHaveLength(VISIBLE_SHORTCUTS.length)

        const text = w.find('.shortcuts-table').text()
        for (const entry of VISIBLE_SHORTCUTS) {
            expect(text).toContain(entry.label)
            for (const key of entry.keys) expect(text).toContain(key)
        }
    })

    it('does not list the overlay-only Escape binding', () => {
        expect(mount(ProfileView).find('.shortcuts-table').text()).not.toContain(
            'Close this overlay'
        )
    })

    it('tells the user which key opens the overlay', () => {
        // The whole point of the section is discoverability, so it has to name
        // the key that brings up the in-player help.
        const text = mount(ProfileView).text()
        expect(text).toContain('?')
    })

    it('renders each key inside a kbd element so it reads as a key', () => {
        const w = mount(ProfileView)
        expect(w.findAll('.shortcut-row kbd').length).toBeGreaterThanOrEqual(
            VISIBLE_SHORTCUTS.length
        )
    })
})
