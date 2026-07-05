<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/store/uiStore'
import { useMusicFolders } from '@/composables/useSubsonicQueries'

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
    { label: 'Now Playing', icon: 'pi pi-play-circle', route: '/', routeName: 'home' },
    { label: 'Search', icon: 'pi pi-search', route: '/search', routeName: 'search' }
]

const { data: musicFolders } = useMusicFolders()

const folderItems = computed<NavItem[]>(() => {
    const folders = musicFolders.value ?? []
    if (folders.length <= 1) {
        return [{ label: 'Library', icon: 'pi pi-headphones', route: '/library', routeName: 'library' }]
    }
    return [
        { label: 'All Music', icon: 'pi pi-headphones', route: '/library', routeName: 'library' },
        ...folders.map((folder) => ({
            label: folder.name,
            icon: 'pi pi-folder',
            route: `/library/${folder.id}`,
            routeName: 'library',
            folderId: folder.id
        }))
    ]
})

const libraryExtras: NavItem[] = [
    { label: 'Playlists', icon: 'pi pi-list', route: '/playlists', routeName: 'playlists' },
    { label: 'Genres', icon: 'pi pi-tags', route: '/genres', routeName: 'genres' }
]

const streamingItems: NavItem[] = [
    { label: 'Podcasts', icon: 'pi pi-microphone', route: '/podcasts', routeName: 'podcasts' },
    { label: 'Radio', icon: 'pi pi-wifi', route: '/radio', routeName: 'radio' }
]

// Avatar identity and Logout are placeholders until the auth system lands.
const bottomItems: NavItem[] = [
    { label: 'Admin Settings', icon: 'pi pi-cog', route: '/admin', routeName: 'admin-settings' },
    { label: 'User Settings', icon: 'pi pi-user-edit', route: '/settings', routeName: 'user-settings' }
]

function logout() {
    console.info('logout placeholder')
}

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
                    <h1 class="logo">Aether</h1>
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
                <i :class="item.icon"></i>
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
                <i :class="item.icon"></i>
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
                <i :class="item.icon"></i>
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
                <i :class="item.icon"></i>
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
                <i :class="item.icon"></i>
                <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </button>

            <div class="nav-separator"></div>

            <button
                class="nav-item"
                @click="logout"
                v-tooltip.right="collapsed ? 'Logout' : undefined"
            >
                <i class="pi pi-sign-out"></i>
                <span v-if="!collapsed" class="nav-label">Logout</span>
            </button>
        </nav>
    </aside>
</template>

<style scoped>
.sidebar {
    width: var(--app-sidebar-width);
    height: 100%;
    background-color: var(--app-surface);
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
    padding: 0 0 0.75rem;
    gap: 0.25rem;
    flex: 1;
    overflow-y: auto;
}

.nav-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1.5rem;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
    font-weight: 500;
    transition: background-color 0.2s, color 0.2s;
    white-space: nowrap;
    text-align: left;
    width: 100%;
}

.sidebar.collapsed .nav-item {
    justify-content: center;
    padding: 0.75rem;
}

.nav-item:hover {
    background-color: var(--app-background);
    color: var(--app-text-primary);
}

.nav-item.active {
    background-color: #eef2ff;
    color: var(--app-accent);
    box-shadow: inset -3px 0 0 var(--app-accent);
}

.nav-item.active:hover {
    background-color: #e0e7ff;
}

.nav-item.sub-item {
    padding-left: 2.5rem;
    font-size: 0.85rem;
}

.nav-item i {
    font-size: 1.1rem;
    flex-shrink: 0;
}

.sidebar-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.5rem 0.5rem 1.5rem;
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
    color: var(--app-text-secondary);
    cursor: pointer;
    border-radius: 50%;
    flex-shrink: 0;
    transition: background-color 0.15s, color 0.15s;
}

.collapse-btn:hover {
    background-color: var(--app-background);
    color: var(--app-text-primary);
}

.header-content {
    flex: 1;
    min-width: 0;
}

.logo {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--app-accent);
    margin: 0;
    white-space: nowrap;
}

.sidebar-footer-nav {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    flex-shrink: 0;
    padding: 0.75rem 0;
    border-top: 1px solid var(--app-border);
}

.nav-separator {
    margin: 0.75rem 0 0.25rem;
}

.sidebar-footer-nav .nav-separator {
    margin: 0.25rem 1.5rem;
    border-top: 1px solid var(--app-border);
}

.sidebar.collapsed .sidebar-footer-nav .nav-separator {
    margin: 0.25rem 0.75rem;
}

.nav-separator.has-label {
    padding-left: 1.5rem;
}

.nav-section-label {
    display: block;
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--app-text-secondary);
}

@media (max-width: 768px) {
    .sidebar {
        display: none;
    }
}
</style>
