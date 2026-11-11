import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import PlaybackLogPanel from '@/components/about/PlaybackLogPanel.vue'
import { clearPlaybackLog, logPlayback } from '@/lib/playbackLog'

const mountPanel = () => mount(PlaybackLogPanel, { global: { plugins: [PrimeVue] } })

beforeEach(() => {
    clearPlaybackLog()
})

describe('PlaybackLogPanel', () => {
    it('says so when nothing is recorded, with its buttons disabled', () => {
        const wrapper = mountPanel()
        expect(wrapper.find('.log-empty').exists()).toBe(true)
        expect(wrapper.find('.playback-log-copy').attributes('disabled')).toBeDefined()
        expect(wrapper.find('.playback-log-clear').attributes('disabled')).toBeDefined()
    })

    it('lists the entries newest first', () => {
        logPlayback('load', 'tr-1')
        logPlayback('error', 'tr-1 code 2')
        const wrapper = mountPanel()
        const events = wrapper.findAll('.log-event').map((e) => e.text())
        expect(events).toEqual(['error', 'load'])
        expect(wrapper.find('.log-detail').text()).toBe('tr-1 code 2')
    })

    it('copies the log oldest first', async () => {
        const writeText = vi.fn(() => Promise.resolve())
        vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } })
        logPlayback('load', 'tr-1')
        logPlayback('ended')
        const wrapper = mountPanel()

        await wrapper.find('.playback-log-copy').trigger('click')
        await flushPromises()

        const text = (writeText.mock.calls[0] as unknown as [string])[0]
        expect(text.split('\n')).toHaveLength(2)
        expect(text).toMatch(/load tr-1\n.* ended$/)
        expect(wrapper.find('.playback-log-copy').text()).toContain('Copied')
        vi.unstubAllGlobals()
    })

    it('clears', async () => {
        logPlayback('load')
        const wrapper = mountPanel()
        await wrapper.find('.playback-log-clear').trigger('click')
        expect(wrapper.find('.log-empty').exists()).toBe(true)
    })
})
