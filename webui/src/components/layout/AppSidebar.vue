<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import { useUiStore } from '@/store/uiStore'
import { useMusicFolders } from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useTheme } from '@/composables/useTheme'
import UserMenu from '@/components/layout/UserMenu.vue'
import BrandMark from '@/components/common/BrandMark.vue'

const route = useRoute()
const router = useRouter()
const uiStore = useUiStore()

interface NavItem {
    label: string
    icon: string
    route: string
    routeName: string
    mode?: 'discover' | 'releases' | 'artists' | 'songs'
    folderId?: number
    shortcut?: string
}

const topItems: NavItem[] = [
    { label: 'Now Playing', icon: 'pi pi-play-circle', route: '/', routeName: 'home', shortcut: 'now-playing' },
    { label: 'Search', icon: 'pi pi-search', route: '/search', routeName: 'search', shortcut: 'search' }
]

// The four browse modes are one route (/library) addressed by hash; only Discover
// carries the `D` badge (all four share routeName 'library').
const libraryModes: NavItem[] = [
    { label: 'Discover', icon: 'pi pi-compass', route: '/library', routeName: 'library', mode: 'discover', shortcut: 'library' },
    { label: 'Releases', icon: 'pi pi-images', route: '/library#releases', routeName: 'library', mode: 'releases' },
    { label: 'Artists', icon: 'pi pi-users', route: '/library#artists', routeName: 'library', mode: 'artists' },
    { label: 'Songs', icon: 'pi pi-wave-pulse', route: '/library#songs', routeName: 'library', mode: 'songs' }
]

const playlistsItem: NavItem = {
    label: 'Playlists', icon: 'pi pi-list', route: '/playlists', routeName: 'playlists', shortcut: 'playlists'
}

const { data: musicFolders } = useMusicFolders()

const folderItems = computed<NavItem[]>(() => {
    const folders = musicFolders.value ?? []
    if (folders.length <= 1) return []
    return folders.map((folder) => ({
        label: folder.name,
        icon: `pi pi-${folder.icon || 'folder'}`,
        route: `/library/${folder.id}`,
        routeName: 'library',
        folderId: folder.id
    }))
})

// The Library block: browse modes, Playlists, then per-library entries.
const libraryGroup = computed<NavItem[]>(() => [...libraryModes, playlistsItem, ...folderItems.value])

const streamingItems: NavItem[] = [
    { label: 'Genres', icon: 'pi pi-tags', route: '/genres', routeName: 'genres', shortcut: 'genres' },
    { label: 'Radio', icon: 'pi pi-wifi', route: '/radio', routeName: 'radio', shortcut: 'radio' }
]

const currentMode = computed<'discover' | 'releases' | 'artists' | 'songs'>(() => {
    const h = route.hash.replace('#', '')
    return h === 'releases' || h === 'artists' || h === 'songs' ? h : 'discover'
})

const isActive = (item: NavItem): boolean => {
    if (item.routeName === 'home') return route.name === 'home'
    if (item.routeName === 'library') {
        if (route.name !== 'library') return false
        const raw = route.params.folderId
        const currentFolder = Array.isArray(raw) ? raw[0] : raw
        const currentId = currentFolder ? Number(currentFolder) : undefined
        if (item.folderId !== undefined) return item.folderId === currentId
        // Root browse-mode entry: active only at the cross-collection root and on
        // the matching hash mode.
        return currentId === undefined && item.mode === currentMode.value
    }
    return route.path.startsWith(item.route)
}

const navigateTo = (item: NavItem) => {
    router.push(item.route)
}

const collapsed = computed(() => uiStore.sidebarCollapsed)

// --- Brand: the header doubles as the "take me back" destination --------------
// With something queued the natural home is Now Playing; with nothing queued
// that view has nothing to show, so the library is the useful landing spot.
const { queue } = usePlayer()

const brandLabel = computed(() =>
    queue.value.length > 0 ? 'Aether — go to Now Playing' : 'Aether — go to Library'
)

const goHome = (): void => {
    router.push(queue.value.length > 0 ? '/' : '/library')
}

// --- Easter egg: the diamond mark unlocks the hidden themes -------------------
// Five clicks inside EGG_WINDOW_MS reveal them in User settings and switch
// to the first; every further burst cycles to the next. The burst window means
// idle curiosity (a stray click) never trips it.
const EGG_CLICKS = 5
const EGG_WINDOW_MS = 1500

const toast = useToast()
const { hiddenUnlocked, unlockHiddenThemes, cycleHiddenTheme } = useTheme()

const eggClicks = ref(0)
let eggTimer: ReturnType<typeof setTimeout> | undefined

const resetEgg = (): void => {
    clearTimeout(eggTimer)
    eggTimer = undefined
    eggClicks.value = 0
}

// Deliberately does not stop propagation: the mark still navigates like the rest
// of the brand, so the trigger stays indistinguishable from ordinary header
// clicks. The repeated same-route pushes it causes are no-ops.
const onBrandMarkClick = (): void => {
    clearTimeout(eggTimer)
    eggClicks.value += 1

    if (eggClicks.value < EGG_CLICKS) {
        eggTimer = setTimeout(resetEgg, EGG_WINDOW_MS)
        return
    }

    resetEgg()
    // Read before unlocking so the toast can tell first discovery from a cycle.
    const firstUnlock = !hiddenUnlocked.value
    unlockHiddenThemes()
    const theme = cycleHiddenTheme()
    toast.add({
        severity: 'success',
        summary: firstUnlock ? 'Hidden themes unlocked' : `Theme: ${theme.label}`,
        detail: firstUnlock
            ? `${theme.label} enabled — the rest live in User settings.`
            : undefined,
        life: 4000
    })
}

onBeforeUnmount(resetEgg)
</script>

<template>
    <aside class="sidebar" :class="{ collapsed }">
        <div class="sidebar-header">
            <template v-if="!collapsed">
                <div class="header-content">
                    <!-- The whole brand is the way back: Now Playing when there
                         is a queue, the library when there is not. -->
                    <button
                        class="brand"
                        type="button"
                        :aria-label="brandLabel"
                        @click="goHome"
                    >
                        <!-- The diamond is also the easter-egg trigger. It stays
                             a decorative image inside the button so it is
                             neither separately focusable nor announced —
                             advertising it would stop it being hidden. -->
                        <BrandMark size="1.75rem" @click="onBrandMarkClick" />
                        <span class="logo">A<span class="logo-accent">e</span>ther</span>
                    </button>
                </div>
            </template>
            <button
                class="collapse-btn"
                type="button"
                :aria-label="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
                v-tooltip.right="collapsed ? 'Expand' : undefined"
                @click="uiStore.toggleSidebar"
            >
                <i :class="collapsed ? 'pi pi-angle-right' : 'pi pi-angle-left'"></i>
            </button>
        </div>

        <nav class="sidebar-nav">
            <!-- Top: the two primary destinations, each carrying its shortcut badge. -->
            <button
                v-for="item in topItems"
                :key="item.routeName"
                class="nav-item"
                :data-shortcut="item.shortcut"
                :class="{ active: isActive(item) }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <i :class="item.icon"></i>
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>

            <div class="nav-separator"></div>
            <div v-if="!collapsed" class="nav-section-label">Library</div>

            <!-- Library block: the four browse modes (one route, hash-addressed),
                 Playlists, then any per-folder entries. Only Discover and Playlists
                 carry shortcut badges. -->
            <button
                v-for="item in libraryGroup"
                :key="item.route"
                class="nav-item"
                :data-shortcut="item.shortcut"
                :class="{ active: isActive(item) }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <i :class="item.icon"></i>
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>

            <div class="nav-separator"></div>

            <button
                v-for="item in streamingItems"
                :key="item.routeName"
                class="nav-item"
                :data-shortcut="item.shortcut"
                :class="{ active: isActive(item) }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <i :class="item.icon"></i>
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>
        </nav>

        <!-- The identity chip: opens the account popup (user settings, theme,
             settings, log out). Replaces the old footer Settings nav entry. -->
        <div class="sidebar-footer-nav">
            <UserMenu :collapsed="collapsed" />
        </div>
    </aside>
</template>

<style scoped>
.sidebar {
    width: var(--app-sidebar-width);
    height: 100%;
    background-color: var(--app-nav-bg);
    /* Depth: a deeper tone at the top easing into an accent-tinted glow at the
       bottom, layered over the base nav colour (accent adapts to the theme). */
    background-image: linear-gradient(
        180deg,
        rgba(0, 0, 0, 0.14) 0%,
        transparent 62%,
        color-mix(in srgb, var(--app-accent) 10%, transparent) 100%
    );
    display: flex;
    flex-direction: column;
    transition: width 0.3s ease;
    flex-shrink: 0;
    overflow: hidden;
}

.sidebar.collapsed {
    width: var(--app-sidebar-collapsed-width);
}

.sidebar-nav {
    display: flex;
    flex-direction: column;
    padding: 1.5rem 0 0.75rem;
    gap: 0.25rem;
    flex: 1;
    overflow-y: auto;
}

.nav-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.55rem 1rem;
    border: none;
    border-radius: 0;
    background: none;
    cursor: pointer;
    color: var(--app-nav-text-dim);
    font-size: 0.85rem;
    font-weight: 600;
    transition: background-color 0.2s, color 0.2s;
    white-space: nowrap;
    text-align: left;
    width: 100%;
}

.sidebar.collapsed .nav-item {
    justify-content: center;
    padding: 0.55rem;
}

.nav-item:hover {
    background-color: rgba(255, 255, 255, 0.06);
    color: var(--app-nav-text);
}

.nav-item.active {
    background-color: var(--app-accent-soft);
    color: var(--app-accent);
    box-shadow: inset -4px 0 0 var(--app-accent);
}

.nav-item.active:hover {
    background-color: var(--app-accent-soft-hover);
}

.nav-item i {
    font-size: 1.1rem;
    flex-shrink: 0;
}

.nav-section-label {
    padding: 0.75rem 1rem 0.25rem;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--app-nav-text-dim);
}

.sidebar-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 1.25rem 0.5rem 0.75rem 1rem;
    min-height: 3rem;
    box-sizing: border-box;
    flex-shrink: 0;
}

.sidebar.collapsed .sidebar-header {
    justify-content: center;
    padding: 0.5rem;
}

.collapse-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    border: none;
    background: transparent;
    color: var(--app-nav-text-dim);
    cursor: pointer;
    border-radius: 50%;
    flex-shrink: 0;
    transition: background-color 0.15s, color 0.15s;
}

.collapse-btn:hover {
    background-color: rgba(255, 255, 255, 0.06);
    color: var(--app-nav-text);
}

.header-content {
    flex: 1;
    min-width: 0;
}

.brand {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0;
    border: none;
    background: none;
    font: inherit;
    color: inherit;
    cursor: pointer;
}

.logo {
    font-size: 1.5rem;
    font-weight: 800;
    letter-spacing: 0.02em;
    color: var(--app-nav-brand);
    margin: 0;
    white-space: nowrap;
}

.logo-accent {
    color: var(--app-nav-brand-alt);
}

.sidebar-footer-nav {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    flex-shrink: 0;
    padding: 0.5rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.nav-separator {
    margin: 0.75rem 0 0.25rem;
}
</style>
