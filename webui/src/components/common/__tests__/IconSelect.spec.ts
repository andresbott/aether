import { describe, it, expect } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

import IconSelect from '@/components/common/IconSelect.vue'

// Real <transition>s and a real document: PrimeVue's Popover fires @show and
// @hide from its transition hooks, and Vue Test Utils' default transition stub
// never runs them — a stubbed harness would report nothing either way.
const mountSelect = () =>
    mount(IconSelect, {
        props: { modelValue: 'folder' },
        attachTo: document.body,
        global: { plugins: [PrimeVue], stubs: { teleport: true, transition: false } }
    })

describe('IconSelect', () => {
    // A Dialog hosting this picker needs to know the Popover is open so it can
    // stop closing on Escape while it is — see LibraryDialog's iconPickerOpen.
    it('reports its popover open and closed', async () => {
        const w = mountSelect()
        await flushPromises()
        expect(w.emitted('update:open')).toBeUndefined()

        await w.get('.icon-select-trigger').trigger('click')
        await flushPromises()
        expect(w.emitted('update:open')).toEqual([[true]])

        // Picking an icon hides the popover, so the host hears "closed" again.
        await w.findAll('.icon-item')[0].trigger('click')
        await flushPromises()
        expect(w.emitted('update:open')).toEqual([[true], [false]])
        w.unmount()
    })
})
