import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LibraryIcon from '@/components/common/LibraryIcon.vue'

describe('LibraryIcon', () => {
    it('masks with the embedded SVG for its name', () => {
        const w = mount(LibraryIcon, { props: { name: 'queue_music' } })
        expect(w.classes()).toContain('app-icon')
        expect(w.attributes('data-icon')).toBe('queue_music')
        expect(w.attributes('style')).toContain('/icons/ms/queue_music.svg')
        expect(w.attributes('aria-hidden')).toBe('true')
    })
    it.each([undefined, '', 'folder-open', 'Folder', '../etc'])('falls back to folder for %s', (name) => {
        const w = mount(LibraryIcon, { props: { name } })
        expect(w.attributes('data-icon')).toBe('folder')
    })
})
