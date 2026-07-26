import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import FileUpload from 'primevue/fileupload'
import HeroHeader from '@/components/layout/HeroHeader.vue'

const mountHero = (props: Record<string, unknown> = {}, slots: Record<string, string> = {}) =>
    mount(HeroHeader, {
        props: { eyebrow: 'Playlist', editing: false, ...props },
        slots: {
            read: '<h2 class="hero-name">Late Night</h2>',
            edit: '<input class="edit-name" />',
            ...slots
        },
        global: { plugins: [PrimeVue] }
    })

describe('HeroHeader', () => {
    it('renders the eyebrow and the cover image when a url is given', () => {
        const w = mountHero({ coverUrl: '/cover/x?size=250' })
        expect(w.find('.eyebrow').text()).toBe('Playlist')
        expect(w.find('.flip-front img').attributes('src')).toBe('/cover/x?size=250')
        expect(w.find('.cover-placeholder').exists()).toBe(false)
    })

    it('renders a placeholder icon when there is no cover', () => {
        const w = mountHero({ coverUrl: null, coverPlaceholderIcon: 'pi pi-user' })
        expect(w.find('.flip-front img').exists()).toBe(false)
        expect(w.find('.cover-placeholder .pi.pi-user').exists()).toBe(true)
    })

    it('flips the cover and marks the root editing when editing is true', () => {
        const w = mountHero({ editing: true })
        expect(w.find('.hero-header').classes()).toContain('editing')
        expect(w.find('.hero-cover').classes()).toContain('flipped')
    })

    it('does not flip while read-only, and keeps the cover a flat 2D layer', () => {
        const w = mountHero({ editing: false })
        const cover = w.find('.hero-cover')
        expect(cover.classes()).not.toContain('flipped')
        // No 3D context when not editing → no mount/navigation flip artifact.
        expect(cover.classes()).not.toContain('active')
    })

    it('renders both read and edit slot content', () => {
        const w = mountHero()
        expect(w.find('.read-only .hero-name').exists()).toBe(true)
        expect(w.find('.edit-only .edit-name').exists()).toBe(true)
    })

    it('renders the #actions slot in read mode', () => {
        const w = mountHero(
            { editing: false },
            { actions: '<button class="hero-act-probe">go</button>' }
        )
        expect(w.find('.hero-act-probe').exists()).toBe(true)
    })

    it('hides the #actions slot in edit mode', () => {
        const w = mountHero(
            { editing: true },
            { actions: '<button class="hero-act-probe">go</button>' }
        )
        expect(w.find('.hero-act-probe').exists()).toBe(false)
    })

    it('emits cover-select with the picked file', () => {
        const w = mountHero()
        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        expect(w.emitted('cover-select')?.[0]).toEqual([file])
    })

    it('emits cover-remove when Remove is clicked', async () => {
        const w = mountHero()
        await w.find('.cover-remove').trigger('click')
        expect(w.emitted('cover-remove')).toHaveLength(1)
    })

    it('omits the cover controls when coverEditable is false', () => {
        const w = mountHero({ coverEditable: false })
        expect(w.find('.flip-back').exists()).toBe(false)
        expect(w.findComponent(FileUpload).exists()).toBe(false)
    })
})

describe('HeroHeader cover controls', () => {
    it('labels the picker as an upload action', () => {
        const w = mountHero({ editing: true })
        expect(w.findComponent(FileUpload).props('chooseLabel')).toBe('Upload image')
    })

    // PrimeVue's own file label says "No file chosen" until a file is picked,
    // which contradicts an image that is already on the server. The #cover-note
    // slot carries that state instead, so the built-in label is suppressed.
    it('suppresses PrimeVue\'s built-in file label', () => {
        const w = mountHero({ editing: true })
        expect(w.text()).not.toContain('No file chosen')
        expect(w.find('.p-fileupload-filelabel').exists()).toBe(false)
    })

    // The picked filename belongs on its own row, not squeezed alongside the
    // button inside the 250px panel.
    it('stacks the picker and its status on separate rows', () => {
        const w = mountHero({ editing: true })
        expect(w.find('.cover-controls').exists()).toBe(true)
    })
})

// A flat secondary text button reads as disabled next to the solid upload
// button. Remove is a real, destructive action — it has to look clickable.
describe('HeroHeader remove button prominence', () => {
    it('renders Remove as an outlined danger button, not muted text', () => {
        const w = mountHero({ editing: true })
        const btn = w.find('.cover-remove')
        expect(btn.exists()).toBe(true)
        // Not PrimeVue's flat/text treatment...
        expect(btn.classes()).not.toContain('p-button-text')
        // ...and carries the danger severity so it is visibly actionable.
        expect(btn.classes()).toContain('p-button-danger')
    })
})
