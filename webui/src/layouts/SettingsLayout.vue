<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/store/uiStore'

interface SettingsNavItem {
    label: string
    icon: string
    route?: string
    action?: () => void
}
interface SettingsNavGroup {
    label: string
    items: SettingsNavItem[]
}

const route = useRoute()
const router = useRouter()
const uiStore = useUiStore()

// Placeholder until the auth system lands.
function logout() {
    console.info('logout placeholder')
}

const groups: SettingsNavGroup[] = [
    {
        label: 'Account',
        items: [
            { label: 'Profile', icon: 'pi pi-user', route: '/settings/profile' },
            { label: 'Logout', icon: 'pi pi-sign-out', action: logout }
        ]
    },
    {
        label: 'Administration',
        items: [
            { label: 'Libraries', icon: 'pi pi-folder', route: '/settings/libraries' },
            { label: 'Tasks', icon: 'pi pi-clock', route: '/settings/tasks' },
            { label: 'Metadata Editor', icon: 'pi pi-pencil', route: '/settings/metadata' }
        ]
    }
]

const collapsed = computed(() => uiStore.settingsSidebarCollapsed)

const isActive = (item: SettingsNavItem): boolean =>
    item.route ? route.path.startsWith(item.route) : false
const onNavItem = (item: SettingsNavItem) => {
    if (item.action) item.action()
    else if (item.route) router.push(item.route)
}
const goBack = () => router.push('/')

onMounted(() => {
    uiStore.checkScreenWidth()
    window.addEventListener('resize', uiStore.checkScreenWidth)
})

onUnmounted(() => {
    window.removeEventListener('resize', uiStore.checkScreenWidth)
})
</script>

<template>
    <div class="settings-layout">
        <aside class="settings-sidebar" :class="{ collapsed }">
            <div class="sidebar-header">
                <h1 v-if="!collapsed" class="settings-title">Settings</h1>
                <button
                    class="collapse-btn"
                    type="button"
                    :aria-label="collapsed ? 'Expand settings sidebar' : 'Collapse settings sidebar'"
                    v-tooltip.right="collapsed ? 'Expand' : undefined"
                    @click="uiStore.toggleSettingsSidebar"
                >
                    <i :class="collapsed ? 'pi pi-angle-right' : 'pi pi-angle-left'"></i>
                </button>
            </div>

            <nav class="sidebar-nav">
                <template v-for="(group, gi) in groups" :key="group.label">
                    <div v-if="!collapsed" class="nav-section-label">{{ group.label }}</div>
                    <div v-else-if="gi > 0" class="nav-separator"></div>
                    <button
                        v-for="item in group.items"
                        :key="item.label"
                        class="nav-item"
                        :class="{ active: isActive(item) }"
                        type="button"
                        @click="onNavItem(item)"
                        v-tooltip.right="collapsed ? item.label : undefined"
                    >
                        <i :class="item.icon"></i>
                        <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
                    </button>
                </template>
            </nav>

            <nav class="sidebar-footer-nav">
                <button
                    class="nav-item"
                    type="button"
                    @click="goBack"
                    v-tooltip.right="collapsed ? 'Back to player' : undefined"
                >
                    <i class="pi pi-arrow-left"></i>
                    <span v-if="!collapsed" class="nav-label">Back to player</span>
                </button>
            </nav>
        </aside>

        <main class="settings-content">
            <RouterView />
        </main>
    </div>
</template>

<style scoped>
.settings-layout {
    display: flex;
    height: 100vh;
    width: 100%;
    background-color: var(--app-background);
    overflow: hidden;
}

.settings-sidebar {
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

.settings-sidebar.collapsed {
    width: var(--app-sidebar-collapsed-width);
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

.settings-sidebar.collapsed .sidebar-header {
    justify-content: center;
    padding: 0.5rem;
}

.settings-title {
    flex: 1;
    font-size: 1.25rem;
    font-weight: 800;
    letter-spacing: 0.02em;
    color: var(--app-nav-brand);
    margin: 0;
    white-space: nowrap;
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

.settings-sidebar.collapsed .nav-item {
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
    display: block;
    padding: 0 1rem;
    margin: 0.75rem 0 0.25rem;
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--app-nav-text-dim);
}

.nav-separator {
    margin: 0.75rem 1rem 0.25rem;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.sidebar-footer-nav {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    flex-shrink: 0;
    padding: 0.75rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.settings-content {
    flex: 1;
    overflow: hidden;
    min-height: 0;
}
</style>
