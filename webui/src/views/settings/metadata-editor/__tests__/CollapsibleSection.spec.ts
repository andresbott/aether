import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CollapsibleSection from '@/views/settings/metadata-editor/CollapsibleSection.vue'

function mountSection(props: Record<string, unknown> = {}) {
    return mount(CollapsibleSection, {
        props: { title: 'Song', ...props },
        slots: {
            default: '<div class="body-content">fields</div>',
            actions: '<button class="action-btn">Act</button>'
        }
    })
}

describe('CollapsibleSection', () => {
    it('renders expanded by default with title, actions and body', () => {
        const wrapper = mountSection()
        expect(wrapper.find('h4').text()).toContain('Song')
        expect(wrapper.find('.action-btn').exists()).toBe(true)
        expect(wrapper.find('.body-content').isVisible()).toBe(true)
        expect(wrapper.find('[data-test="section-toggle"]').attributes('aria-expanded')).toBe(
            'true'
        )
    })

    it('toggles the body on click, keeping it in the DOM (v-show)', async () => {
        const wrapper = mountSection()
        const bodyStyle = () =>
            wrapper.find('[data-test="section-body"]').attributes('style') ?? ''

        await wrapper.find('[data-test="section-toggle"]').trigger('click')
        // Hidden but still mounted: collapsed inputs keep their edit buffers.
        expect(wrapper.find('.body-content').exists()).toBe(true)
        expect(bodyStyle()).toContain('display: none')
        expect(wrapper.find('[data-test="section-toggle"]').attributes('aria-expanded')).toBe(
            'false'
        )

        await wrapper.find('[data-test="section-toggle"]').trigger('click')
        expect(bodyStyle()).not.toContain('display: none')
        expect(wrapper.find('[data-test="section-toggle"]').attributes('aria-expanded')).toBe(
            'true'
        )
    })

    it('shows the help icon only when help text is given', () => {
        expect(mountSection().find('[data-test="section-help"]').exists()).toBe(false)
        expect(
            mountSection({ help: 'What this section does' })
                .find('[data-test="section-help"]')
                .exists()
        ).toBe(true)
    })

    it('applies the staged accent class when dirty', () => {
        const wrapper = mountSection({ dirty: true })
        expect(wrapper.find('.edit-section').classes()).toContain('section-dirty')
    })
})
