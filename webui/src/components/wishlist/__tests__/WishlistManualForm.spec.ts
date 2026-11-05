import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import SelectButton from 'primevue/selectbutton'
import WishlistManualForm from '@/components/wishlist/WishlistManualForm.vue'

const mountForm = () => mount(WishlistManualForm, { global: { plugins: [PrimeVue] } })

describe('WishlistManualForm', () => {
    it('needs a title before it can add', async () => {
        const w = mountForm()
        expect(w.find('.manual-add').attributes('disabled')).toBeDefined()
        await w.find('.manual-title').setValue('   ')
        await w.find('form').trigger('submit')
        expect(w.emitted('add')).toBeUndefined()
    })

    it('emits the type, the trimmed title and the description', async () => {
        const w = mountForm()
        w.findComponent(SelectButton).vm.$emit('update:modelValue', 'song')
        await w.find('.manual-title').setValue('  Radio session  ')
        await w.find('.manual-description').setValue(' 1993, BBC ')
        await w.find('form').trigger('submit')
        expect(w.emitted('add')?.[0]).toEqual([{ kind: 'song', title: 'Radio session', note: '1993, BBC' }])
    })

    it('defaults to an album with an empty description', async () => {
        const w = mountForm()
        await w.find('.manual-title').setValue('Demos')
        await w.find('form').trigger('submit')
        expect(w.emitted('add')?.[0]).toEqual([{ kind: 'album', title: 'Demos', note: '' }])
    })
})
