import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import HeroVisibilityBar from '@/components/layout/HeroVisibilityBar.vue'

const mountBar = (isPublic = false) =>
    mount(HeroVisibilityBar, {
        props: { modelValue: isPublic },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

describe('HeroVisibilityBar', () => {
    it('labels the toggle "Private" when the playlist is not public', () => {
        expect(mountBar(false).find('.ev-toggle').text()).toContain('Private')
    })

    it('labels the toggle "Public" when the playlist is public', () => {
        expect(mountBar(true).find('.ev-toggle').text()).toContain('Public')
    })

    it('emits the flipped value when the toggle is clicked', async () => {
        const w = mountBar(false)
        await w.find('.ev-toggle').trigger('click')
        expect(w.emitted('update:modelValue')).toEqual([[true]])
    })

    it('shows only the toggle, with no descriptive hint text', () => {
        expect(mountBar(false).find('.ev-hint').exists()).toBe(false)
    })
})
