<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/store/uiStore'

const route = useRoute()
const router = useRouter()
const uiStore = useUiStore()

interface NavItem {
    label: string
    icon: string
    route: string
    routeName: string
}

const navItems: NavItem[] = [
    { label: 'Now Playing', icon: 'pi pi-play-circle', route: '/', routeName: 'home' },
    { label: 'Library', icon: 'pi pi-headphones', route: '/library', routeName: 'library' },
    { label: 'Playlists', icon: 'pi pi-list', route: '/playlists', routeName: 'playlists' },
    { label: 'Podcasts', icon: 'pi pi-microphone', route: '/podcasts', routeName: 'podcasts' },
    { label: 'Radio', icon: 'pi pi-wifi', route: '/radio', routeName: 'radio' }
]

const isActive = (item: NavItem): boolean => {
    if (item.routeName === 'home') return route.name === 'home'
    return route.path.startsWith(item.route)
}

const navigateTo = (item: NavItem) => {
    router.push(item.route)
}

const collapsed = computed(() => uiStore.sidebarCollapsed)
</script>

<template>
    <aside class="sidebar" :class="{ collapsed }">
        <nav class="sidebar-nav">
            <button
                v-for="item in navItems"
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

        <button
            class="collapse-btn"
            type="button"
            :aria-label="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
            v-tooltip.right="collapsed ? 'Expand' : undefined"
            @click="uiStore.toggleSidebar"
        >
            <i :class="collapsed ? 'pi pi-angle-right' : 'pi pi-angle-left'"></i>
            <span v-if="!collapsed" class="collapse-label">Collapse</span>
        </button>
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
    padding: 0.75rem 0.5rem;
    gap: 0.25rem;
    flex: 1;
    overflow-y: auto;
}

.nav-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    border: none;
    background: none;
    cursor: pointer;
    border-radius: 8px;
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
}

.nav-item i {
    font-size: 1.1rem;
    flex-shrink: 0;
}

.collapse-btn {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--app-text-secondary);
    border-top: 1px solid var(--app-border);
    transition: background-color 0.2s, color 0.2s;
    width: 100%;
    text-align: left;
}

.sidebar.collapsed .collapse-btn {
    justify-content: center;
    padding: 0.75rem;
    gap: 0;
}

.collapse-btn:hover {
    background-color: var(--app-background);
    color: var(--app-text-primary);
}

.collapse-btn i {
    font-size: 1rem;
    flex-shrink: 0;
}

.collapse-label {
    font-size: 0.9rem;
    font-weight: 500;
}

@media (max-width: 768px) {
    .sidebar {
        display: none;
    }
}
</style>
