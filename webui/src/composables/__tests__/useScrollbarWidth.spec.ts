import { describe, it, expect } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'

function withComposable() {
    const captured: { width?: ReturnType<typeof useScrollbarWidth> } = {}
    const Host = defineComponent({
        setup() {
            captured.width = useScrollbarWidth()
            return () => h('div')
        }
    })
    const wrapper = mount(Host)
    return { captured, wrapper }
}

describe('useScrollbarWidth', () => {
    it('returns a non-negative number and leaves no probe element behind', () => {
        const before = document.body.children.length
        const { captured } = withComposable()
        expect(typeof captured.width!.value).toBe('number')
        expect(captured.width!.value).toBeGreaterThanOrEqual(0)
        // The probe div is removed synchronously after measuring.
        expect(document.body.children.length).toBe(before)
    })
})
