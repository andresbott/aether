import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import HeroEditSelectionBar from '@/components/layout/HeroEditSelectionBar.vue'

const mountBar = (count = 2) =>
    mount(HeroEditSelectionBar, {
        props: { count },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

describe('HeroEditSelectionBar', () => {
    it('shows the selected count', () => {
        expect(mountBar(3).find('.es-count').text()).toContain('3 selected')
    })

    it('emits move-top, move-bottom, delete and clear from its buttons', async () => {
        const w = mountBar()
        await w.find('.es-top').trigger('click')
        await w.find('.es-bottom').trigger('click')
        await w.find('.es-delete').trigger('click')
        await w.find('.es-clear').trigger('click')
        expect(w.emitted('move-top')).toHaveLength(1)
        expect(w.emitted('move-bottom')).toHaveLength(1)
        expect(w.emitted('delete')).toHaveLength(1)
        expect(w.emitted('clear')).toHaveLength(1)
    })
})
