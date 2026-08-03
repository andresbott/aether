import { describe, it, expect, vi } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'

vi.mock('vue-router', () => ({
    useRoute: () => ({ name: 'library', path: '/library', params: {} }),
    useRouter: () => ({ push: vi.fn() })
}))
// Two folders, so the per-folder entries actually render — `folderItems` is empty
// below two, and those entries share `routeName: 'library'` with the root, which
// is exactly the collision the anchor has to avoid.
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({
        data: ref([
            { id: 1, name: 'Main' },
            { id: 2, name: 'Archive' }
        ])
    })
}))
vi.mock('@/store/uiStore', () => ({
    useUiStore: () => ({ sidebarCollapsed: false, toggleSidebar: vi.fn() })
}))
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ queue: ref([]) }) }))
vi.mock('@/composables/useTheme', () => ({
    useTheme: () => ({
        hiddenUnlocked: ref(false),
        unlockHiddenThemes: vi.fn(),
        cycleHiddenTheme: vi.fn()
    })
}))

import AppSidebar from '@/components/layout/AppSidebar.vue'
import { SHORTCUTS } from '@/utils/shortcuts'

// The other half of the anchor partition: PlayerControls.shortcutAnchors.spec.ts
// asserts every above-placed anchor, this file every side-placed one. Derived from
// the registry so a newly added nav shortcut fails here until it is anchored.
const SIDE_ANCHORS = SHORTCUTS.filter((s) => s.place === 'side').map((s) => s.anchor as string)

// The nav shortcuts have no control in the player bar, so their badges are pinned
// to the sidebar entries that open them. Without these anchors the keys would only
// be discoverable from the settings list.
describe('AppSidebar shortcut anchors', () => {
    const mountSidebar = () => mount(AppSidebar, { global: { directives: { tooltip: {} } } })

    it.each([
        ['now-playing', 'Now Playing'],
        ['library', 'Library'],
        ['search', 'Search'],
        ['playlists', 'Playlists'],
        ['genres', 'Genres'],
        ['radio', 'Radio']
    ])('anchors the %s shortcut to its nav entry', (anchor, label) => {
        const el = mountSidebar().find(`[data-shortcut="${anchor}"]`)
        expect(el.exists()).toBe(true)
        expect(el.text()).toContain(label)
    })

    it('anchors every side-placed shortcut in the registry', () => {
        const w = mountSidebar()
        for (const anchor of SIDE_ANCHORS) {
            expect(
                w.find(`[data-shortcut="${anchor}"]`).exists(),
                `missing sidebar anchor: ${anchor}`
            ).toBe(true)
        }
    })

    it.each(['now-playing', 'library', 'search', 'playlists', 'genres', 'radio'])(
        'gives %s exactly one anchor',
        (anchor) => {
            expect(mountSidebar().findAll(`[data-shortcut="${anchor}"]`)).toHaveLength(1)
        }
    )

    // Six shortcuts, six anchors: three primary entries plus the three browse
    // destinations below the separator. The PER-FOLDER entries in between carry
    // none, so a bare count is what catches an anchor leaking onto them.
    it('anchors exactly the six nav entries that have shortcuts', () => {
        expect(mountSidebar().findAll('.sidebar-nav .nav-item[data-shortcut]')).toHaveLength(6)
    })

    // The per-folder entries share `routeName: 'library'` with the root entry, so
    // a naive routeName test would anchor every one of them and the overlay would
    // badge whichever it found first. The `D` badge belongs on the cross-collection
    // root — the entry with no folderId.
    it('anchors the library root, not the per-folder entries', () => {
        const el = mountSidebar().find('[data-shortcut="library"]')
        expect(el.text()).toContain('Library')
        expect(el.text()).not.toContain('Main')
    })

    // The footer nav (Settings) has no shortcut, so the attribute must not leak
    // out of the primary group.
    it('leaves the footer nav unanchored', () => {
        expect(mountSidebar().findAll('.sidebar-footer-nav [data-shortcut]')).toHaveLength(0)
    })
})
