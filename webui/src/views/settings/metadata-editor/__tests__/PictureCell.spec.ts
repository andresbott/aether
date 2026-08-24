import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import PictureCell from '@/views/settings/metadata-editor/PictureCell.vue'

const mountCell = (props: Record<string, unknown> = {}) =>
    mount(PictureCell, {
        props: {
            image: 'http://img/x.jpg',
            title: 'Album folder',
            changeTestId: 'change',
            removeTestId: 'remove',
            undoTestId: 'undo',
            metaTestId: 'meta',
            ...props
        },
        global: { plugins: [PrimeVue] }
    })

describe('PictureCell', () => {
    it('renders the image and an info column with title, subtitle, note and meta', () => {
        const w = mountCell({
            subtitle: 'artist “X”',
            note: 'cover.jpg',
            meta: '1400 × 1400 · JPEG · 500 KB'
        })
        // The image lives in the art area; the text lives in a separate info column.
        expect(w.find('.cell-art img.cell-thumb').attributes('src')).toBe('http://img/x.jpg')
        const info = w.find('.cell-info')
        expect(info.exists()).toBe(true)
        expect(info.text()).toContain('Album folder')
        expect(info.text()).toContain('artist “X”')
        expect(info.text()).toContain('cover.jpg')
        expect(w.find('[data-test="meta"]').text()).toBe('1400 × 1400 · JPEG · 500 KB')
    })

    it('flips an occupied removable cell to Change and Remove', async () => {
        const w = mountCell({ canRemove: true })
        expect(w.find('.cell-flip').exists()).toBe(true)
        await w.find('[data-test="change"]').trigger('click')
        await w.find('[data-test="remove"]').trigger('click')
        expect(w.emitted('change')).toHaveLength(1)
        expect(w.emitted('remove')).toHaveLength(1)
    })

    it('hides Remove when the cell is not removable', () => {
        expect(mountCell({ canRemove: false }).find('[data-test="remove"]').exists()).toBe(false)
    })

    it('shows an add button and no image when empty', async () => {
        const w = mountCell({ image: null })
        expect(w.find('img.cell-thumb').exists()).toBe(false)
        await w.find('[data-test="change"]').trigger('click')
        expect(w.emitted('change')).toHaveLength(1)
    })

    it('shows only Undo when a change is staged', async () => {
        const w = mountCell({ staged: true, pending: true })
        expect(w.find('[data-test="change"]').exists()).toBe(false)
        expect(w.find('[data-test="remove"]').exists()).toBe(false)
        await w.find('[data-test="undo"]').trigger('click')
        expect(w.emitted('undo')).toHaveLength(1)
    })

    it('marks the card pending or removing', () => {
        expect(mountCell({ pending: true }).find('.picture-cell').classes()).toContain('pending')
        expect(
            mountCell({ image: null, removing: true, staged: true })
                .find('.picture-cell')
                .classes()
        ).toContain('removing')
    })

    it('omits the meta line when no meta is given', () => {
        expect(mountCell({}).find('[data-test="meta"]').exists()).toBe(false)
    })

    it('labels the on-image controls with text, not just icons', () => {
        const occupied = mountCell({ canRemove: true })
        expect(occupied.find('[data-test="change"]').text()).toContain('Change')
        expect(occupied.find('[data-test="remove"]').text()).toContain('Remove')

        const staged = mountCell({ staged: true, pending: true })
        expect(staged.find('[data-test="undo"]').text()).toContain('Undo')

        const empty = mountCell({ image: null, addLabel: 'Add image' })
        expect(empty.find('[data-test="change"]').text()).toContain('Add image')
    })
})
