<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import Toast from 'primevue/toast'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import PlayerControls from '@/components/layout/PlayerControls.vue'
import QueueSidebar from '@/components/layout/QueueSidebar.vue'
import SearchBar from '@/components/layout/SearchBar.vue'
import { useUiStore } from '@/store/uiStore'

const uiStore = useUiStore()

onMounted(() => {
    uiStore.checkScreenWidth()
    window.addEventListener('resize', uiStore.checkScreenWidth)
})
</script>

<template>
    <div class="app-container">
        <AppSidebar />

        <div class="content-area">
            <div class="content-main">
                <header class="top-bar">
                    <SearchBar />
                </header>
                <main class="main-content">
                    <RouterView />
                </main>
            </div>

            <QueueSidebar />
        </div>

        <PlayerControls />
        <Toast />
    </div>
</template>

<style>
.app-container {
    display: flex;
    height: 100vh;
    width: 100%;
    overflow: hidden;
    background-color: var(--app-background);
}

.content-area {
    display: flex;
    flex: 1;
    height: 100vh;
    overflow: hidden;
}

.content-main {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    height: 100vh;
}

.top-bar {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: 0.75rem 2rem;
    border-bottom: 1px solid var(--app-border);
    flex-shrink: 0;
}

.main-content {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    background-color: var(--app-background);
    padding: 2rem;
}

.view-placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    min-height: 50vh;
    color: var(--app-text-secondary);
}

.view-placeholder h1 {
    font-size: 2rem;
    margin-bottom: 0.5rem;
    color: var(--app-text-primary);
}

@media (max-width: 768px) {
    .content-area {
        flex-direction: column;
    }

    .main-content {
        padding: 1rem;
    }
}
</style>
