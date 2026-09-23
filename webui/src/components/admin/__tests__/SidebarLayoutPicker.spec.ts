import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SidebarLayoutPicker from '@/components/admin/SidebarLayoutPicker.vue'

const mountPicker = (modelValue: boolean) =>
    mount(SidebarLayoutPicker, { props: { modelValue, name: 'layout' } })

describe('SidebarLayoutPicker', () => {
    it('offers the two layouts as captioned cards, marking the current one', () => {
        const w = mountPicker(true)
        const cards = w.findAll('.layout-card')
        expect(cards.map((c) => c.find('.layout-caption').text())).toEqual(['One entry', 'Entry per view'])
        expect(cards.map((c) => c.classes('selected'))).toEqual([false, true])
    })

    // The miniatures differ where the layouts do: one entry switches views in
    // the page header, an entry per view lists them in the sidebar.
    it('draws the view switch in the header for one entry and in the sidebar per view', () => {
        const [one, split] = mountPicker(false).findAll('.mock')
        expect(one.findAll('.mock-switch .pill')).toHaveLength(3)
        expect(one.findAll('.mock-sidebar .bar:not(.label)')).toHaveLength(1)
        expect(split.find('.mock-switch').exists()).toBe(false)
        expect(split.findAll('.mock-sidebar .bar:not(.label)')).toHaveLength(3)
    })

    it('emits the picked layout', async () => {
        const w = mountPicker(false)
        await w.findAll('input[type="radio"]')[1].setValue(true)
        expect(w.emitted('update:modelValue')).toEqual([[true]])
    })
})
