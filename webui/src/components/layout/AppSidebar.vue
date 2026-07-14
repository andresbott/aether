<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/store/uiStore'
import { useMusicFolders } from '@/composables/useSubsonicQueries'
import NavIcon from './NavIcon.vue'

const route = useRoute()
const router = useRouter()
const uiStore = useUiStore()

interface NavItem {
    label: string
    icon: string
    route: string
    routeName: string
    folderId?: number
}

const primaryItems: NavItem[] = [
    { label: 'Now Playing', icon: 'now-playing', route: '/', routeName: 'home' },
    { label: 'Search', icon: 'search', route: '/search', routeName: 'search' }
]

const { data: musicFolders } = useMusicFolders()

const folderItems = computed<NavItem[]>(() => {
    const folders = musicFolders.value ?? []
    if (folders.length <= 1) {
        return [{ label: 'Library', icon: 'music', route: '/library', routeName: 'library' }]
    }
    return [
        { label: 'All Music', icon: 'music', route: '/library', routeName: 'library' },
        ...folders.map((folder) => ({
            label: folder.name,
            icon: 'folder',
            route: `/library/${folder.id}`,
            routeName: 'library',
            folderId: folder.id
        }))
    ]
})

const libraryExtras: NavItem[] = [
    { label: 'Playlists', icon: 'list', route: '/playlists', routeName: 'playlists' },
    { label: 'Genres', icon: 'tag', route: '/genres', routeName: 'genres' }
]

const streamingItems: NavItem[] = [
    { label: 'Podcasts', icon: 'mic', route: '/podcasts', routeName: 'podcasts' },
    { label: 'Radio', icon: 'radio', route: '/radio', routeName: 'radio' }
]

const bottomItems: NavItem[] = [
    { label: 'Settings', icon: 'settings', route: '/settings', routeName: 'settings' }
]

const isActive = (item: NavItem): boolean => {
    if (item.routeName === 'home') return route.name === 'home'
    if (item.routeName === 'library') {
        if (route.name !== 'library') return false
        const raw = route.params.folderId
        const currentFolder = Array.isArray(raw) ? raw[0] : raw
        const currentId = currentFolder ? Number(currentFolder) : undefined
        return item.folderId === currentId
    }
    return route.path.startsWith(item.route)
}

const navigateTo = (item: NavItem) => {
    router.push(item.route)
}

const collapsed = computed(() => uiStore.sidebarCollapsed)
</script>

<template>
    <aside class="sidebar" :class="{ collapsed }">
        <div class="sidebar-header">
            <template v-if="!collapsed">
                <div class="header-content">
                    <div class="brand">
                        <span class="brand-mark">◈</span>
                        <h1 class="logo">Aether</h1>
                    </div>
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
            <button
                v-for="item in primaryItems"
                :key="item.routeName"
                class="nav-item"
                :class="{ active: isActive(item) }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <NavIcon :name="item.icon" />
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>

            <div class="nav-separator" :class="{ 'has-label': !collapsed }">
                <span v-if="!collapsed" class="nav-section-label">Library</span>
            </div>

            <button
                v-for="item in folderItems"
                :key="item.route"
                class="nav-item"
                :class="{ active: isActive(item), 'sub-item': item.folderId !== undefined }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <NavIcon :name="item.icon" />
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>

            <button
                v-for="item in libraryExtras"
                :key="item.route"
                class="nav-item"
                :class="{ active: isActive(item) }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <NavIcon :name="item.icon" />
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>

            <div class="nav-separator" :class="{ 'has-label': !collapsed }">
                <span v-if="!collapsed" class="nav-section-label">Streaming</span>
            </div>

            <button
                v-for="item in streamingItems"
                :key="item.routeName"
                class="nav-item"
                :class="{ active: isActive(item) }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <NavIcon :name="item.icon" />
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>
        </nav>

        <nav class="sidebar-footer-nav">
            <button
                v-for="item in bottomItems"
                :key="item.routeName"
                class="nav-item"
                :class="{ active: isActive(item) }"
                @click="navigateTo(item)"
                v-tooltip.right="collapsed ? item.label : undefined"
            >
                <NavIcon :name="item.icon" />
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>
        </nav>
    </aside>
</template>

<style scoped>
.sidebar {
    width: var(--app-sidebar-width);
    height: 100%;
    background-color: var(--app-nav-bg);
    border-right: 1px solid var(--app-border);
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

.nav-item.sub-item {
    padding-left: 2.5rem;
    font-size: 0.85rem;
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
}

.brand-mark {
    color: var(--app-accent);
    font-size: 1.625rem;
    line-height: 1;
}

.logo {
    font-size: 1.5rem;
    font-weight: 800;
    letter-spacing: 0.02em;
    color: var(--app-nav-brand);
    margin: 0;
    white-space: nowrap;
}

.sidebar-footer-nav {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    flex-shrink: 0;
    padding: 0.75rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.nav-separator {
    margin: 0.75rem 0 0.25rem;
}

.sidebar-footer-nav .nav-separator {
    margin: 0.25rem 1rem;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.sidebar.collapsed .sidebar-footer-nav .nav-separator {
    margin: 0.25rem 0.75rem;
}

.nav-separator.has-label {
    padding-left: 1rem;
}

.nav-section-label {
    display: block;
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--app-nav-text-dim);
}

@media (max-width: 768px) {
    .sidebar {
        display: none;
    }
}
</style>
