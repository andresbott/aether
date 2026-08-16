import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import {
    resetNowPlayingSheetForTests,
    useNowPlayingSheet
} from '@/composables/useNowPlayingSheet'

const queue = ref<Array<Record<string, unknown>>>([{ id: '1' }, { id: '2' }, { id: '3' }])
const shuffle = ref(false)
const repeat = ref<'none' | 'all' | 'one'>('none')
const toggleShuffle = vi.fn()
const toggleRepeat = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue, shuffle, repeat, toggleShuffle, toggleRepeat })
}))

vi.mock('@/composables/useQueueSummary', () => ({
    useQueueSummary: () => ({ trackCount: ref(3), summary: ref('3 tracks • 3 min') })
}))

const openSaveDialog = vi.fn()
const clearQueue = vi.fn()
vi.mock('@/composables/useQueueActions', () => ({
    useQueueActions: () => ({
        showSaveDialog: ref(false),
        playlistName: ref(''),
        openSaveDialog,
        handleSave: vi.fn(),
        isSaving: ref(false),
        clearQueue
    })
}))

vi.mock('@/components/layout/QueueBody.vue', () => ({
    default: {
        name: 'QueueBody',
        props: ['variant', 'editMode'],
        template: '<div class="stub-queue-body">{{ variant }}</div>'
    }
}))

vi.mock('@/components/layout/SavePlaylistDialog.vue', () => ({
    default: { name: 'SavePlaylistDialog', template: '<div class="stub-save-dialog"></div>' }
}))

import QueuePanel from '@/components/layout/QueuePanel.vue'

const mountPanel = () =>
    mount(QueuePanel, { global: { plugins: [PrimeVue], directives: { tooltip: {} } } })

// The Popover teleports to document.body and mounted wrappers accumulate
// across tests, so always take the LAST match — that is this test's panel.
const overflowAction = (selector: string): HTMLElement => {
    const all = document.body.querySelectorAll<HTMLElement>(selector)
    const el = all[all.length - 1]
    expect(el).toBeTruthy()
    return el
}
const openOverflow = async (w: ReturnType<typeof mountPanel>) => {
    await w.find('.queue-heading-actions .queue-overflow-btn').trigger('click')
}

beforeEach(() => {
    resetNowPlayingSheetForTests()
    useNowPlayingSheet().snapTo('queue')
    shuffle.value = false
    repeat.value = 'none'
    vi.clearAllMocks()
})

describe('QueuePanel', () => {
    it('renders the queue heading with the shared summary', () => {
        const w = mountPanel()
        expect(w.find('.queue-heading h2').text()).toBe('Queue')
        expect(w.find('.queue-heading-summary').text()).toBe('3 tracks • 3 min')
        expect(w.find('.stub-queue-body').text()).toBe('sidebar')
    })

    it('shuffle and repeat sit in the heading, wired to the player', async () => {
        const w = mountPanel()
        const actions = w.find('.queue-heading-actions')
        await actions.find('[aria-label="Shuffle"]').trigger('click')
        expect(toggleShuffle).toHaveBeenCalledOnce()
        await actions.find('[aria-label="Repeat"]').trigger('click')
        expect(toggleRepeat).toHaveBeenCalledOnce()
    })

    it('shuffle and repeat read their pressed state from the player', () => {
        shuffle.value = true
        repeat.value = 'all'
        const w = mountPanel()
        expect(w.find('.queue-action-shuffle').attributes('aria-pressed')).toBe('true')
        expect(w.find('.queue-action-shuffle').classes()).toContain('is-active')
        expect(w.find('.queue-action-repeat').attributes('aria-pressed')).toBe('true')
        expect(w.find('.queue-action-repeat').classes()).toContain('is-active')
    })

    it('edit, save and clear collapse behind the heading ⋮ menu', async () => {
        const w = mountPanel()
        expect(w.find('.queue-heading-actions .queue-action-save').exists()).toBe(false)
        await openOverflow(w)
        const save = overflowAction('.queue-action-save')
        expect(save.textContent).toContain('Save as playlist')
        save.click()
        expect(openSaveDialog).toHaveBeenCalledOnce()
        overflowAction('.queue-action-clear').click()
        expect(clearQueue).toHaveBeenCalledOnce()
    })

    it('the pencil in the ⋮ menu toggles edit mode on the queue body', async () => {
        const w = mountPanel()
        const body = w.findComponent({ name: 'QueueBody' })
        expect(body.props('editMode')).toBe(false)
        await openOverflow(w)
        overflowAction('.queue-action-edit').click()
        await w.vm.$nextTick()
        expect(body.props('editMode')).toBe(true)
    })

    // Edit mode is queue-panel UI: leaving the queue detent — by swipe, hint
    // or back button, all of which move the sheet — ends the session, so
    // returning never lands on a stale selection.
    it('leaving the queue detent exits edit mode', async () => {
        const w = mountPanel()
        await openOverflow(w)
        overflowAction('.queue-action-edit').click()
        await w.vm.$nextTick()
        expect(w.findComponent({ name: 'QueueBody' }).props('editMode')).toBe(true)

        useNowPlayingSheet().snapTo('playing')
        await w.vm.$nextTick()
        expect(w.findComponent({ name: 'QueueBody' }).props('editMode')).toBe(false)
    })
})
