<script setup lang="ts">
// Avatar identity and Logout are placeholders until the auth system lands.
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'

const router = useRouter()
const menu = ref<InstanceType<typeof Menu> | null>(null)
const menuOpen = ref(false)

const items: MenuItem[] = [
    {
        label: 'Admin Settings',
        icon: 'pi pi-cog',
        command: () => router.push('/admin')
    },
    {
        label: 'User Settings',
        icon: 'pi pi-user-edit',
        command: () => router.push('/settings')
    },
    { separator: true },
    {
        label: 'Logout',
        icon: 'pi pi-sign-out',
        command: () => console.info('logout placeholder')
    }
]

function toggle(event: Event) {
    menu.value?.toggle(event)
}

function onShow() {
    menuOpen.value = true
}

function onHide() {
    menuOpen.value = false
}
</script>

<template>
    <div class="user-menu">
        <button
            class="avatar-btn"
            type="button"
            aria-haspopup="true"
            :aria-expanded="menuOpen"
            aria-controls="user-menu-popup"
            aria-label="User menu"
            @click="toggle"
        >
            <i class="pi pi-user"></i>
        </button>
        <Menu
            id="user-menu-popup"
            ref="menu"
            :model="items"
            :popup="true"
            @show="onShow"
            @hide="onHide"
        />
    </div>
</template>

<style scoped>
.user-menu {
    display: flex;
    align-items: center;
}

.avatar-btn {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    border: none;
    background-color: var(--app-accent);
    color: #fff;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: filter 0.15s;
    padding: 0;
}

.avatar-btn:hover {
    filter: brightness(0.9);
}

.avatar-btn:focus-visible {
    outline: 2px solid var(--app-accent);
    outline-offset: 2px;
}

.avatar-btn i {
    font-size: 1rem;
}
</style>
