import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { ref } from 'vue'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))
vi.mock('@/composables/useSubsonicQueries', () => ({
    useTogglePlaylistStar: () => ({ mutate: vi.fn() })
}))
const currentUser = ref({ login: 'alice', role: 'user' })
vi.mock('@/composables/useAuth', () => ({ useAuth: () => ({ currentUser }) }))

import PlaylistRow from '@/components/library/PlaylistRow.vue'
import type { Playlist } from '@/types/subsonic'

const playlist: Playlist = {
    id: 'pl1',
    name: 'Road trip',
    owner: 'alice',
    songCount: 12,
    duration: 2580,
    created: '2026-09-01T12:00:00Z',
    changed: '2026-10-08T12:00:00Z'
}
const played = '2026-10-17T12:00:00Z'

const mountRow = (p: Playlist) =>
    mount(PlaylistRow, {
        props: { playlist: p },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

const partsOf = (w: ReturnType<typeof mountRow>, line: string) =>
    w.findAll(`${line} .meta-part`).map((p) => p.text())

beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-18T12:00:00Z'))
})

afterEach(() => {
    vi.useRealTimers()
})

describe('PlaylistRow', () => {
    it('shows the description as the subtitle, when there is one', () => {
        const described = mountRow({ ...playlist, comment: '  Long drives  ' })
        expect(described.find('.card-subtitle').text()).toBe('Long drives')
        expect(mountRow(playlist).find('.card-subtitle').exists()).toBe(false)
    })

    it('shows the length, then when it last changed and was last played', () => {
        const w = mountRow({ ...playlist, played })
        expect(partsOf(w, '.card-meta')).toEqual(['12 songs', '43 min'])
        expect(partsOf(w, '.card-details')).toEqual(['updated 10 days ago', 'played yesterday'])
        expect(partsOf(mountRow(playlist), '.card-details')).toEqual(['updated 10 days ago'])
    })

    it("names the owner of someone else's playlist first, beside the view-only lock", () => {
        const parts = mountRow({ ...playlist, owner: 'bob' }).findAll('.card-details .meta-part')
        expect(parts[0].text()).toBe('by bob')
        const lock = parts[0].find('.ms-lock')
        expect(lock.attributes('role')).toBe('img')
        expect(lock.attributes('aria-label')).toBe('Shared by bob — view only')
        expect(mountRow(playlist).find('.card-facts .ms-lock').exists()).toBe(false)
    })
})
