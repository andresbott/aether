<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
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
        <main class="admin-content">
            <RouterView />
        </main>
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

.admin-content {
    flex: 1;
    overflow-y: auto;
    padding: 2rem;
}
</style>
