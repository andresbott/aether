<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import Popover from 'primevue/popover'
import { useAuth } from '@/composables/useAuth'

// The identity chip is the sidebar's single bottom control: it opens a popup
// with everything personal (user settings, log out) plus the Admin entry to
// the server administration area (/settings).
defineProps<{ collapsed?: boolean }>()

const router = useRouter()

// Identity from the /me bootstrap. With auth method "none" there is no
// session: the chip falls back to "Guest" and the popup has nothing to log
// out of, so the Log out entry is dropped (same gate as SettingsLayout).
// The Admin and Metadata editor entries only exist for admins — the routes
// they open both use the admin-only settings layout, so a non-admin is
// redirected away and the /api/v1 data behind them answers 403.
const { authRequired, currentUser, isAdmin, logout } = useAuth()

const popoverRef = ref<InstanceType<typeof Popover> | null>(null)
const isOpen = ref(false)

const displayName = computed(() => currentUser.value?.login ?? 'Guest')

const toggleMenu = (event: Event) => popoverRef.value?.toggle(event)

const goUserSettings = () => {
    popoverRef.value?.hide()
    router.push('/user-settings')
}

const goSettings = () => {
    popoverRef.value?.hide()
    router.push('/settings')
}

const goMetadataEditor = () => {
    popoverRef.value?.hide()
    router.push('/metadata-editor')
}

const goAbout = () => {
    popoverRef.value?.hide()
    router.push('/about')
}

const onLogout = () => {
    popoverRef.value?.hide()
    logout.mutate()
}
</script>

<template>
    <div class="user-menu" :class="{ collapsed }">
        <button
            class="user-btn"
            type="button"
            aria-haspopup="menu"
            :aria-expanded="isOpen"
            :aria-label="`Account: ${displayName}`"
            v-tooltip.right="collapsed ? displayName : undefined"
            @click="toggleMenu"
        >
            <span class="user-avatar" aria-hidden="true"><i class="pi pi-user"></i></span>
            <span v-if="!collapsed" class="user-name">{{ displayName }}</span>
        </button>

        <Popover
            ref="popoverRef"
            class="user-menu-popover"
            @show="isOpen = true"
            @hide="isOpen = false"
        >
            <div class="account-menu" role="menu" :aria-label="`Account menu for ${displayName}`">
                <button class="menu-item" role="menuitem" type="button" @click="goUserSettings">
                    <i class="pi pi-user"></i>
                    <span>User settings</span>
                </button>
                <button
                    v-if="isAdmin"
                    class="menu-item"
                    role="menuitem"
                    type="button"
                    @click="goSettings"
                >
                    <i class="pi pi-cog"></i>
                    <span>Admin</span>
                </button>
                <button
                    v-if="isAdmin"
                    class="menu-item"
                    role="menuitem"
                    type="button"
                    @click="goMetadataEditor"
                >
                    <i class="pi pi-pencil"></i>
                    <span>Metadata editor</span>
                </button>
                <button class="menu-item" role="menuitem" type="button" @click="goAbout">
                    <i class="pi pi-info-circle"></i>
                    <span>About</span>
                </button>
                <template v-if="authRequired">
                    <div class="menu-sep"></div>
                    <button
                        class="menu-item danger"
                        role="menuitem"
                        type="button"
                        @click="onLogout"
                    >
                        <i class="pi pi-sign-out"></i>
                        <span>Log out</span>
                    </button>
                </template>
            </div>
        </Popover>
    </div>
</template>

<style scoped>
.user-menu {
    display: flex;
    align-items: center;
}

/* Matches .nav-item in AppSidebar so the chip reads as one more entry:
   full width, flush edges, same padding, colors and hover. */
.user-btn {
    flex: 1;
    min-width: 0;
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
    text-align: left;
    transition: background-color 0.2s, color 0.2s;
}

.user-menu.collapsed .user-btn {
    justify-content: center;
    padding: 0.55rem;
}

.user-btn:hover,
.user-btn[aria-expanded='true'] {
    background-color: rgba(255, 255, 255, 0.06);
    color: var(--app-nav-text);
}

.user-avatar {
    width: 1.9rem;
    height: 1.9rem;
    border-radius: 50%;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--app-accent-soft);
    color: var(--app-accent);
}

.user-avatar i {
    font-size: 0.95rem;
}

.user-name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>

<!-- The popover teleports to body, so its content cannot be styled from a
     scoped block (same pattern as IconSelect). -->
<style>
.user-menu-popover.p-popover .p-popover-content {
    padding: 0.375rem;
}

.user-menu-popover .account-menu {
    min-width: 13rem;
}

.user-menu-popover .menu-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
    padding: 0.5rem 0.6rem;
    border: none;
    border-radius: var(--app-radius);
    background: none;
    cursor: pointer;
    color: var(--app-text-primary);
    font: inherit;
    text-align: left;
}

.user-menu-popover .menu-item:hover {
    background: var(--app-hover);
}

.user-menu-popover .menu-item i {
    width: 1.1rem;
    text-align: center;
    flex-shrink: 0;
}

.user-menu-popover .menu-item.danger {
    color: var(--p-red-400, #f87171);
}

.user-menu-popover .menu-sep {
    height: 1px;
    background: var(--app-border);
    margin: 0.375rem 0.25rem;
}
</style>
