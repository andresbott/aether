<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Drawer from 'primevue/drawer'
import { useAuth } from '@/composables/useAuth'
import { useMobileNav } from '@/composables/useMobileNav'
import { usePlayer } from '@/composables/usePlayer'
import { useMusicFolders } from '@/composables/useSubsonicQueries'

// The mobile shell's whole navigation surface, opened by the hamburger in
// ContentScaffold's header. Mirrors AppSidebar's destinations, order, and
// icons so the two shells read as one app, and folds in the account entries
// the desktop keeps behind the UserMenu popup.
const route = useRoute()
const router = useRouter()
const { authRequired, isAdmin, logout } = useAuth()
const { data: musicFolders } = useMusicFolders()
const { isOpen, close } = useMobileNav()
const { queue } = usePlayer()

interface DrawerItem {
    label: string
    icon: string
    route: string
    routeName?: string
    folderId?: number
}

// Now Playing and Queue only exist while there is something queued: with an
// empty queue, `/` replaces itself with the library (see HomeView), so either
// entry would silently take the user somewhere else. Queue is the same route
// addressed to its second panel — `/#queue` scrolls MobilePlayView's snap
// scroller to the queue face — and is drawer-only on purpose: the desktop
// keeps the queue in its sidebar.
const primaryItems = computed<DrawerItem[]>(() => [
    ...(queue.value.length > 0
        ? [
              { label: 'Now Playing', icon: 'pi pi-play-circle', route: '/', routeName: 'home' },
              { label: 'Queue', icon: 'pi pi-list-check', route: '/#queue', routeName: 'queue' }
          ]
        : []),
    { label: 'Library', icon: 'pi pi-compass', route: '/library', routeName: 'library' },
    { label: 'Search', icon: 'pi pi-search', route: '/search', routeName: 'search' }
])

// Same rule as AppSidebar: a single library needs no entry of its own (the
// Library entry already covers it), so this is empty below two.
const folderItems = computed<DrawerItem[]>(() => {
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

const browseItems: DrawerItem[] = [
    { label: 'Playlists', icon: 'pi pi-list', route: '/playlists' },
    { label: 'Genres', icon: 'pi pi-tags', route: '/genres' },
    { label: 'Radio', icon: 'pi pi-wifi', route: '/radio' }
]

// Same order as UserMenu's popup so the two account surfaces read alike:
// User settings → Admin (admins only) → About.
const accountItems = computed<DrawerItem[]>(() => {
    const items: DrawerItem[] = [
        { label: 'User settings', icon: 'pi pi-user', route: '/user-settings' }
    ]
    if (isAdmin.value) items.push({ label: 'Admin', icon: 'pi pi-cog', route: '/settings' })
    items.push({ label: 'About', icon: 'pi pi-info-circle', route: '/about' })
    return items
})

// Same discrimination as AppSidebar: the library root and the per-folder
// entries share a route name, so the folderId decides which one lights up.
const isActive = (item: DrawerItem): boolean => {
    // Now Playing and Queue share the home route; the hash names the panel,
    // and MobilePlayView keeps it in sync with the swiped-to panel.
    if (item.routeName === 'home') return route.name === 'home' && route.hash !== '#queue'
    if (item.routeName === 'queue') return route.name === 'home' && route.hash === '#queue'
    if (item.routeName === 'library') {
        if (route.name !== 'library') return false
        const raw = route.params.folderId
        const currentFolder = Array.isArray(raw) ? raw[0] : raw
        const currentId = currentFolder ? Number(currentFolder) : undefined
        return item.folderId === currentId
    }
    return route.path.startsWith(item.route)
}

// The drawer outlives navigation (it is shell chrome), so its own item
// handlers are not enough: a system-back press with the drawer open pops a
// route entry UNDERNEATH it. Any route change closes the drawer, so
// navigation is never covered by a stale overlay.
watch(
    () => route.fullPath,
    () => close()
)

const go = (item: DrawerItem): void => {
    close()
    void router.push(item.route)
}

const onLogout = (): void => {
    close()
    logout.mutate()
}
</script>

<template>
    <Drawer v-model:visible="isOpen" position="left" class="mobile-nav-drawer">
        <template #header>
            <span class="drawer-brand">
                <span class="drawer-brand-mark">◈</span>
                <span class="drawer-brand-logo">A<span class="drawer-brand-accent">e</span>ther</span>
            </span>
        </template>
        <nav class="drawer-nav" aria-label="Primary">
            <button
                v-for="item in primaryItems"
                :key="item.route"
                type="button"
                class="drawer-item"
                :class="{ active: isActive(item) }"
                :aria-current="isActive(item) ? 'page' : undefined"
                @click="go(item)"
            >
                <i :class="item.icon"></i>
                <span>{{ item.label }}</span>
            </button>
            <div class="drawer-sep"></div>

            <button
                v-for="item in [...folderItems, ...browseItems]"
                :key="item.route"
                type="button"
                class="drawer-item"
                :class="{ active: isActive(item) }"
                :aria-current="isActive(item) ? 'page' : undefined"
                @click="go(item)"
            >
                <i :class="item.icon"></i>
                <span>{{ item.label }}</span>
            </button>
            <div class="drawer-sep"></div>

            <button
                v-for="item in accountItems"
                :key="item.route"
                type="button"
                class="drawer-item"
                @click="go(item)"
            >
                <i :class="item.icon"></i>
                <span>{{ item.label }}</span>
            </button>

            <template v-if="authRequired">
                <div class="drawer-sep"></div>
                <button type="button" class="drawer-item danger" @click="onLogout">
                    <i class="pi pi-sign-out"></i>
                    <span>Log out</span>
                </button>
            </template>
        </nav>
    </Drawer>
</template>

<style scoped>
.drawer-brand {
    display: flex;
    align-items: center;
    gap: 0.625rem;
}

.drawer-brand-mark {
    color: var(--app-accent);
    font-size: 1.5rem;
    line-height: 1;
}

.drawer-brand-logo {
    font-size: 1.35rem;
    font-weight: 800;
    letter-spacing: 0.02em;
    color: var(--app-text-primary);
    white-space: nowrap;
}

.drawer-brand-accent {
    color: var(--app-accent);
}

.drawer-nav {
    display: flex;
    flex-direction: column;
}

.drawer-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    /* Horizontal inset lives here, not on the drawer content box, so the
       item's background and accent bar span the drawer's full width. */
    padding: 0.85rem 1.25rem;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--app-text-primary);
    font-size: 0.95rem;
    text-align: left;
    width: 100%;
    /* Square with an accent edge bar, matching AppSidebar's .nav-item.active
       so the two nav surfaces read as one. */
    border-radius: 0;
}

.drawer-item:hover {
    background-color: var(--app-hover);
}

.drawer-item.active {
    background-color: var(--app-accent-soft);
    color: var(--app-accent);
    box-shadow: inset -4px 0 0 var(--app-accent);
}

.drawer-item.danger {
    color: var(--p-red-400, #f87171);
}

.drawer-sep {
    height: 1px;
    margin: 0.35rem 0;
    background-color: var(--app-border);
}
</style>
