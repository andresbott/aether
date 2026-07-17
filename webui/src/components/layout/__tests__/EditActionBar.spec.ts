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
