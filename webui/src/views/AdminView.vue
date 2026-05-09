<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'

const navItems = [
    { label: 'Libraries', to: { name: 'admin-libraries' } },
    { label: 'Tasks', to: { name: 'admin-tasks' } },
    { label: 'Metadata Editor', to: { name: 'admin-metadata' } },
] as const
</script>

<template>
    <div class="admin-shell">
        <nav class="admin-nav">
            <RouterLink
                v-for="item in navItems"
                :key="item.label"
                :to="item.to"
                class="admin-nav-item"
            >
                {{ item.label }}
            </RouterLink>
        </nav>
        <div class="admin-content">
            <RouterView />
        </div>
    </div>
</template>

<style scoped>
.admin-shell {
    display: flex;
    gap: 2rem;
    max-width: 1200px;
    margin: 0 auto;
    min-height: 100%;
}

.admin-nav {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    width: 180px;
    flex-shrink: 0;
    padding-top: 0.25rem;
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
    background-color: #eef2ff;
    color: var(--app-accent);
}

.admin-content {
    flex: 1;
    min-width: 0;
}
</style>
