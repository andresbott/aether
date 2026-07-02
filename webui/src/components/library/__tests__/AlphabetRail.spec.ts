import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AlphabetRail from '@/components/library/AlphabetRail.vue'

const letters = [
    { name: '#', offset: 0, count: 2 },
    { name: 'A', offset: 2, count: 5 },
    { name: 'C', offset: 7, count: 3 }
]

describe('AlphabetRail', () => {
    it('renders # + A-Z and disables letters with no albums', () => {
        const w = mount(AlphabetRail, { props: { letters } })
        const buttons = w.findAll('button')
        expect(buttons).toHaveLength(27) // # + 26 letters
        const byText = (t: string) => buttons.find((b) => b.text() === t)!
        expect(byText('A').attributes('disabled')).toBeUndefined()
        expect(byText('B').attributes('disabled')).toBeDefined()
        expect(byText('C').attributes('disabled')).toBeUndefined()
    })

    it('emits the offset of an enabled letter on click', async () => {
        const w = mount(AlphabetRail, { props: { letters } })
        const a = w.findAll('button').find((b) => b.text() === 'A')!
        await a.trigger('click')
        expect(w.emitted('select')).toEqual([[2]])
    })

    it('does not emit for a disabled letter', async () => {
        const w = mount(AlphabetRail, { props: { letters } })
        const b = w.findAll('button').find((btn) => btn.text() === 'B')!
        await b.trigger('click')
        expect(w.emitted('select')).toBeUndefined()
    })
})
