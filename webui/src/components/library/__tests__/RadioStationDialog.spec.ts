import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import PrimeVue from 'primevue/config'

// PrimeVue's Dialog teleports its content out of the wrapper; render it inline so
// assertions can see the dialog body (same approach as ArtistEditDialog.spec).
vi.mock('primevue/dialog', () => ({
    default: defineComponent({
        props: ['visible', 'header', 'modal', 'style'],
        emits: ['update:visible'],
        setup(_props, { slots }) {
            return () =>
                h('div', { class: 'p-dialog' }, [
                    h('div', { class: 'p-dialog-content' }, slots.default?.()),
                    h('div', { class: 'p-dialog-footer' }, slots.footer?.())
                ])
        }
    })
}))

import RadioStationDialog from '@/components/library/RadioStationDialog.vue'

describe('RadioStationDialog', () => {
    it('shows a deprecation note pointing at the Settings radio editor', () => {
        const w = mount(RadioStationDialog, {
            props: { visible: true, station: null, submitting: false },
            global: { plugins: [PrimeVue] }
        })
        expect(w.text()).toContain('Deprecated')
        expect(w.text()).toContain('Radio Stations')
    })
})
