<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'

const navItems = [
    { label: 'Libraries', to: { name: 'admin-libraries' } },
    { label: 'Tasks', to: { name: 'admin-tasks' } },
    { label: 'Metadata Editor', to: { name: 'admin-metadata' } },
] as const
</script>

<template>
    <div class="admin-container">
        <header class="admin-topbar">
            <RouterLink to="/" class="admin-logo">Aether</RouterLink>
            <RouterLink to="/" class="back-link">
                <i class="pi pi-arrow-left"></i>
                Back to player
            </RouterLink>
        </header>
        <div class="admin-body">
            <aside class="admin-sidebar">
                <RouterLink
                    v-for="item in navItems"
                    :key="item.label"
                    :to="item.to"
                    class="admin-nav-item"
                >
                    {{ item.label }}
                </RouterLink>
            </aside>
            <main class="admin-content">
                <RouterView />
            </main>
        </div>
    </div>
</template>

<style scoped>
.admin-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100%;
    background-color: var(--app-background);
}

.admin-topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 1.5rem;
    height: 64px;
    background-color: var(--app-surface);
    border-bottom: 1px solid var(--app-border);
    flex-shrink: 0;
}

.admin-logo {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--app-accent);
    text-decoration: none;
}

.back-link {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
    color: var(--app-text-secondary);
    text-decoration: none;
    transition: color 0.15s;
}

.back-link:hover {
    color: var(--app-text-primary);
}

.admin-body {
    display: flex;
    flex: 1;
    min-height: 0;
}

.admin-sidebar {
    width: 280px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    padding: 0.75rem 0.5rem;
    background-color: var(--app-surface);
    border-right: 1px solid var(--app-border);
}

.admin-nav-item {
    display: block;
    padding: 0.6rem 1rem;
    border-radius: 6px;
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--app-text-secondary);
    text-decoration: none;
    transition: background-color 0.15s, color 0.15s;
}

.admin-nav-item:hover {
    background-color: var(--app-border);
    color: var(--app-text-primary);
}

.admin-nav-item.router-link-active {
    background-color: var(--app-accent-soft, #eef2ff);
    color: var(--app-accent);
}

.admin-content {
    flex: 1;
    overflow: hidden;
    min-height: 0;
}
</style>
