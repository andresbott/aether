import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { computed, defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { subsonicClient } from '@/lib/api/subsonic'

const okPing = {
    'subsonic-response': { status: 'ok', version: '1.16.1' }
}

describe('subsonic client apiKey auth', () => {
    beforeEach(() => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve(okPing)
        }))
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        subsonicClient.clearApiKey()
    })

    it('appends apiKey and no u/t/s/p params when a key is set', async () => {
        subsonicClient.setApiKey('aether_abc_secret')
        await subsonicClient.ping()
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.get('apiKey')).toBe('aether_abc_secret')
        for (const p of ['u', 't', 's', 'p']) {
            expect(url.searchParams.has(p), `param ${p}`).toBe(false)
        }
    })

    it('sends no auth params in auth-none mode', async () => {
        subsonicClient.initWithDefaults()
        await subsonicClient.ping()
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.has('apiKey')).toBe(false)
        expect(url.searchParams.has('u')).toBe(false)
    })

    it('getStreamUrl carries the apiKey', () => {
        subsonicClient.setApiKey('aether_abc_secret')
        const url = new URL(subsonicClient.getStreamUrl('tr-1'))
        expect(url.searchParams.get('apiKey')).toBe('aether_abc_secret')
    })

    // The apiKey is held in a ref, so URLs built inside a computed track it: a
    // transparent re-mint must invalidate them rather than leave dead tokens
    // baked into every cover/stream src on the page.
    describe('apiKey reactivity', () => {
        it('re-evaluates a computed cover URL after setApiKey', async () => {
            subsonicClient.setApiKey('aether_old_key')
            const url = computed(() => subsonicClient.getCoverArtUrl('al-1', 200))
            expect(new URL(url.value).searchParams.get('apiKey')).toBe('aether_old_key')

            subsonicClient.setApiKey('aether_new_key')
            await nextTick()
            expect(new URL(url.value).searchParams.get('apiKey')).toBe('aether_new_key')
        })

        // The real consumer chain: AlbumCard's coverUrl computed shape, rendered
        // into the DOM, must repaint with the fresh key.
        it('invalidates a rendered AlbumCard-style cover computed', async () => {
            subsonicClient.setApiKey('aether_old_key')
            const Card = defineComponent({
                setup() {
                    const coverUrl = computed(() => {
                        if (!subsonicClient.isConfigured()) return null
                        return subsonicClient.getCoverArtUrl('al-1', 200)
                    })
                    return { coverUrl }
                },
                template: '<img :src="coverUrl ?? undefined" />'
            })
            const wrapper = mount(Card)
            expect(wrapper.get('img').attributes('src')).toContain('apiKey=aether_old_key')

            subsonicClient.setApiKey('aether_new_key')
            await nextTick()
            expect(wrapper.get('img').attributes('src')).toContain('apiKey=aether_new_key')
            expect(wrapper.get('img').attributes('src')).not.toContain('aether_old_key')
        })
    })
})
