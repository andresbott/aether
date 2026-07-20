import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PicturePickerDialog from '@/components/library/PicturePickerDialog.vue'
import type { CoverCandidate } from '@/types/metadata'

const listReleaseCoversSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', () => ({
    listReleaseCovers: (...args: unknown[]) => listReleaseCoversSpy(...args)
}))

const mkCandidate = (over: Partial<CoverCandidate> = {}): CoverCandidate => ({
    id: '1',
    thumbUrl: 'http://img/1-250.jpg',
    imageUrl: 'http://img/1.jpg',
    isFront: false,
    types: [],
    comment: '',
    ...over
})

const stubs = {
    Dialog: {
        props: ['visible', 'header'],
        template:
            '<div v-if="visible"><div class="dialog-header">{{ header }}</div><slot /><slot name="footer" /></div>'
    },
    Button: {
        props: ['label', 'disabled', 'loading'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')">{{ label }}</button>'
    },
    Message: { template: '<div class="message"><slot /></div>' }
}

function mountPicker(props: Record<string, unknown> = {}) {
    return mount(PicturePickerDialog, {
        props: {
            visible: true,
            pictureType: 'Back Cover',
            pictureSlot: 'folder',
            releaseMbid: 'rel-1',
            releaseGroupMbid: 'rg-1',
            ...props
        },
        global: { stubs }
    })
}

describe('PicturePickerDialog', () => {
    beforeEach(() => {
        listReleaseCoversSpy.mockReset()
        listReleaseCoversSpy.mockResolvedValue([])
    })

    it('does NOT search on open; the search button triggers it', async () => {
        const wrapper = mountPicker()
        await flushPromises()
        expect(listReleaseCoversSpy).not.toHaveBeenCalled()

        await wrapper.find('[data-test="picture-search"]').trigger('click')
        await flushPromises()
        expect(listReleaseCoversSpy).toHaveBeenCalledWith('rel-1', 'rg-1')
    })

    it('disables the search button without release MBIDs', () => {
        const wrapper = mountPicker({ releaseMbid: '', releaseGroupMbid: '' })
        expect(
            wrapper.find('[data-test="picture-search"]').attributes('disabled')
        ).toBeDefined()
    })

    it('shows the type and slot in the header', () => {
        const wrapper = mountPicker({ pictureType: 'Back Cover', pictureSlot: 'db' })
        expect(wrapper.find('.dialog-header').text()).toBe('Change back cover — internal store')
    })

    it('has no save-target radio buttons', () => {
        const wrapper = mountPicker()
        expect(wrapper.text()).not.toContain('Save to')
        expect(wrapper.find('input[type="radio"]').exists()).toBe(false)
    })

    it('sorts candidates matching the requested type first and tags them', async () => {
        listReleaseCoversSpy.mockResolvedValue([
            mkCandidate({ id: 'front', isFront: true, types: ['Front'] }),
            mkCandidate({ id: 'back', types: ['Back'] })
        ])
        const wrapper = mountPicker({ pictureType: 'Back Cover' })
        await wrapper.find('[data-test="picture-search"]').trigger('click')
        await flushPromises()

        const rows = wrapper.findAll('.cover-row')
        expect(rows).toHaveLength(2)
        expect(rows[0].text()).toContain('Back')
        expect(rows[0].find('[data-test="type-match"]').exists()).toBe(true)
        expect(rows[1].find('[data-test="type-match"]').exists()).toBe(false)
    })

    it('emits the selected candidate as {file: null, imageUrl} without persisting', async () => {
        listReleaseCoversSpy.mockResolvedValue([mkCandidate({ id: 'b', types: ['Back'] })])
        const wrapper = mountPicker()
        await wrapper.find('[data-test="picture-search"]').trigger('click')
        await flushPromises()

        await wrapper.find('.cover-row').trigger('click')
        await wrapper.find('[data-test="picture-select"]').trigger('click')

        const emitted = wrapper.emitted('select')
        expect(emitted).toHaveLength(1)
        expect(emitted![0][0]).toEqual({ file: null, imageUrl: 'http://img/1.jpg' })
        expect(wrapper.emitted('update:visible')![0]).toEqual([false])
    })

    it('disables Select until a source is chosen', async () => {
        const wrapper = mountPicker()
        expect(wrapper.find('[data-test="picture-select"]').attributes('disabled')).toBeDefined()
    })

    it('resets search results when reopened', async () => {
        listReleaseCoversSpy.mockResolvedValue([mkCandidate()])
        const wrapper = mountPicker()
        await wrapper.find('[data-test="picture-search"]').trigger('click')
        await flushPromises()
        expect(wrapper.findAll('.cover-row')).toHaveLength(1)

        await wrapper.setProps({ visible: false })
        await wrapper.setProps({ visible: true })
        expect(wrapper.findAll('.cover-row')).toHaveLength(0)
        // ...and it still does not auto-search on the reopen.
        expect(listReleaseCoversSpy).toHaveBeenCalledTimes(1)
    })
})
