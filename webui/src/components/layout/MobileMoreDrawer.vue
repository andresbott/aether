<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import Drawer from 'primevue/drawer'
import { useAuth } from '@/composables/useAuth'
import { useMusicFolders } from '@/composables/useSubsonicQueries'

// Everything the desktop sidebar + UserMenu offer that has no tab of its own:
// per-folder libraries, the browse extras, and the account entries.
const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', value: boolean): void }>()

const router = useRouter()
const { authRequired, isAdmin, logout } = useAuth()
const { data: musicFolders } = useMusicFolders()

interface DrawerItem {
    label: string
    icon: string
    route: string
}

// Same rule as AppSidebar: a single library needs no entry of its own (the
// Library tab already covers it), so this is empty below two.
const folderItems = computed<DrawerItem[]>(() => {
    const folders = musicFolders.value ?? []
    if (folders.length <= 1) return []
    return folders.map((folder) => ({
        label: folder.name,
        icon: `pi pi-${folder.icon || 'folder'}`,
        route: `/library/${folder.id}`
    }))
})

const browseItems: DrawerItem[] = [
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

const close = (): void => emit('update:visible', false)

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
    <Drawer
        :visible="props.visible"
        position="bottom"
        class="mobile-more-drawer app-bottom-sheet"
        header="More"
        @update:visible="emit('update:visible', $event)"
    >
        <nav class="drawer-nav" aria-label="More destinations">
            <button
                v-for="item in folderItems"
                :key="item.route"
                type="button"
                class="drawer-item"
                @click="go(item)"
            >
                <i :class="item.icon"></i>
                <span>{{ item.label }}</span>
            </button>
            <div v-if="folderItems.length" class="drawer-sep"></div>

            <button
                v-for="item in browseItems"
                :key="item.route"
                type="button"
                class="drawer-item"
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
.drawer-nav {
    display: flex;
    flex-direction: column;
}

.drawer-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.85rem 0.5rem;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--app-text-primary);
    font-size: 0.95rem;
    text-align: left;
    width: 100%;
    border-radius: var(--app-radius);
}

.drawer-item:hover {
    background-color: var(--app-hover);
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
