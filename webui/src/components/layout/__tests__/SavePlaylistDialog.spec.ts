import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import PrimeVue from 'primevue/config'

// Mock Dialog to render slots directly for testing
vi.mock('primevue/dialog', () => {
    return {
        default: defineComponent({
            props: ['visible', 'header', 'modal', 'style', 'appendTo'],
            emits: ['update:visible'],
            setup(props, { slots, emit }) {
                return () => {
                    return h('div', { class: 'p-dialog' }, [
                        h('div', { class: 'p-dialog-content' }, slots.default?.()),
                        h('div', { class: 'p-dialog-footer' }, slots.footer?.())
                    ])
                }
            }
        })
    }
})

import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'

const mountDialog = (props: Record<string, unknown>) =>
    mount(SavePlaylistDialog, {
        props: { visible: true, name: '', ...props },
        global: {
            plugins: [PrimeVue]
        }
    })

describe('SavePlaylistDialog', () => {
    it('disables Save when the name is blank', () => {
        const wrapper = mountDialog({ name: '   ' })
        const saveBtn = wrapper.findAll('button').find(b => b.text() === 'Save')
        expect(saveBtn?.attributes('disabled')).toBeDefined()
    })

    it('emits save when the Save button is clicked with a name', async () => {
        const wrapper = mountDialog({ name: 'Road Trip' })
        const saveBtn = wrapper.findAll('button').find(b => b.text() === 'Save')
        await saveBtn?.trigger('click')
        expect(wrapper.emitted('save')).toHaveLength(1)
    })
})
