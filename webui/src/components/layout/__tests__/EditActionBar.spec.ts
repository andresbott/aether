import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const { requireFn } = vi.hoisted(() => {
    const requireFn = vi.fn((opts?: { accept?: () => void }) => {
        if (opts?.accept) {
            opts.accept()
        }
    })
    return { requireFn }
})

vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: requireFn })
}))

import EditActionBar from '@/components/layout/EditActionBar.vue'

const mountBar = (props: Record<string, unknown> = {}, slots: Record<string, string> = {}) =>
    mount(EditActionBar, {
        props: { editing: false, ...props },
        slots,
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

beforeEach(() => requireFn.mockClear())

describe('EditActionBar', () => {
    it('read mode: shows the pencil and read-actions, no edit buttons', () => {
        const w = mountBar({ editing: false }, { 'read-actions': '<button class="play">P</button>' })
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.play').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)
    })

    it('edit mode: renders buttons in order Delete, Save, Cancel', () => {
        // Delete must sit far left so it is not under the cursor (the pencil is the
        // rightmost read-mode button); Cancel takes the rightmost spot.
        const w = mountBar({ editing: true })
        const order = w
            .findAll('button')
            .map((b) => b.classes().find((c) => c.startsWith('edit-action-')))
        expect(order).toEqual(['edit-action-delete', 'edit-action-save', 'edit-action-cancel'])
    })

    it('Escape emits cancel while editing (not dirty: no confirmation)', async () => {
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
        const w = mountBar({ editing: true })
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        expect(confirmSpy).not.toHaveBeenCalled()
        expect(w.emitted('cancel')).toHaveLength(1)
        confirmSpy.mockRestore()
        w.unmount()
    })

    it('Escape verifies unsaved changes when dirty and cancels only if confirmed', () => {
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
        const w = mountBar({ editing: true, dirty: true })
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        expect(confirmSpy).toHaveBeenCalledTimes(1)
        expect(w.emitted('cancel')).toHaveLength(1)
        confirmSpy.mockRestore()
        w.unmount()
    })

    it('Escape does not cancel when the unsaved-changes prompt is dismissed', () => {
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
        const w = mountBar({ editing: true, dirty: true })
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        expect(confirmSpy).toHaveBeenCalledTimes(1)
        expect(w.emitted('cancel')).toBeUndefined()
        confirmSpy.mockRestore()
        w.unmount()
    })

    it('Escape does nothing in read mode', () => {
        const w = mountBar({ editing: false })
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        expect(w.emitted('cancel')).toBeUndefined()
        w.unmount()
    })

    it('Escape does not exit edit mode while a confirm dialog is open', () => {
        const w = mountBar({ editing: true })
        const dialog = document.createElement('div')
        dialog.className = 'p-confirmdialog'
        document.body.appendChild(dialog)
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        expect(w.emitted('cancel')).toBeUndefined()
        document.body.removeChild(dialog)
        w.unmount()
    })

    it('detaches the Escape listener when leaving edit mode', async () => {
        const w = mountBar({ editing: true })
        await w.setProps({ editing: false })
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        expect(w.emitted('cancel')).toBeUndefined()
        w.unmount()
    })

    it('pencil emits update:editing true', async () => {
        const w = mountBar({ editing: false })
        await w.find('.edit-action-edit').trigger('click')
        expect(w.emitted('update:editing')?.[0]).toEqual([true])
    })

    it('edit mode: shows save/cancel/delete and hides read-actions and pencil', () => {
        const w = mountBar({ editing: true }, { 'read-actions': '<button class="play">P</button>' })
        expect(w.find('.edit-action-save').exists()).toBe(true)
        expect(w.find('.edit-action-cancel').exists()).toBe(true)
        expect(w.find('.edit-action-delete').exists()).toBe(true)
        expect(w.find('.play').exists()).toBe(false)
        expect(w.find('.edit-action-edit').exists()).toBe(false)
    })

    it('omits delete when canDelete is false', () => {
        const w = mountBar({ editing: true, canDelete: false })
        expect(w.find('.edit-action-delete').exists()).toBe(false)
    })

    it('save reflects saveDisabled and emits save', async () => {
        const w = mountBar({ editing: true, saveDisabled: true })
        expect(w.find('.edit-action-save').attributes('disabled')).toBeDefined()
        await w.setProps({ saveDisabled: false })
        await w.find('.edit-action-save').trigger('click')
        expect(w.emitted('save')).toHaveLength(1)
    })

    it('cancel emits cancel', async () => {
        const w = mountBar({ editing: true })
        await w.find('.edit-action-cancel').trigger('click')
        expect(w.emitted('cancel')).toHaveLength(1)
    })

    it('delete routes through confirm and emits delete on accept', async () => {
        const w = mountBar({ editing: true, deleteHeader: 'Delete X?', deleteMessage: 'Gone.' })
        await w.find('.edit-action-delete').trigger('click')
        expect(requireFn).toHaveBeenCalledWith(expect.objectContaining({ header: 'Delete X?', message: 'Gone.' }))
        expect(w.emitted('delete')).toHaveLength(1)
    })

    it('does not emit delete when the confirmation is dismissed', async () => {
        requireFn.mockImplementationOnce(() => {})
        const w = mountBar({ editing: true })
        await w.find('.edit-action-delete').trigger('click')
        expect(requireFn).toHaveBeenCalled()
        expect(w.emitted('delete')).toBeUndefined()
    })
})
