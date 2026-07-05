<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import Toast from 'primevue/toast'
import AppTopbar from '@/components/layout/AppTopbar.vue'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import PlayerControls from '@/components/layout/PlayerControls.vue'
import QueueSidebar from '@/components/layout/QueueSidebar.vue'
import { useUiStore } from '@/store/uiStore'

const uiStore = useUiStore()
const route = useRoute()

onMounted(() => {
    uiStore.checkScreenWidth()
    window.addEventListener('resize', uiStore.checkScreenWidth)
})

onUnmounted(() => {
    window.removeEventListener('resize', uiStore.checkScreenWidth)
})
</script>

<template>
    <div class="app-container">
        <AppTopbar />

        <div class="body-row">
            <AppSidebar />

            <div class="content-area">
                <main
                    class="main-content"
                    :class="{ 'main-content--flush': route.meta.flush }"
                >
                    <RouterView />
                </main>
                <QueueSidebar v-if="route.name !== 'home'" />
            </div>
        </div>

        <PlayerControls />
        <Toast />
    </div>
</template>

<style>
.app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100%;
    overflow: hidden;
    background-color: var(--app-background);
    padding-bottom: var(--app-player-height);
}

.body-row {
    display: flex;
    flex: 1;
    min-height: 0;
    overflow: hidden;
}

.content-area {
    display: flex;
    flex: 1;
    min-width: 0;
    overflow: hidden;
}

.main-content {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    background-color: var(--app-background);
    padding: 1rem 2rem;
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
    .main-content {
        padding: 1rem;
    }
}

/* Full-bleed routes (those with `meta: { flush: true }`) manage their own
   horizontal gutter on their header/content, so drop the side padding here and
   let their internal scroll area reach the content-area edge — putting the
   scroll bar flush right. Declared last so it also wins inside the media query
   above. */
.main-content--flush {
    padding-left: 0;
    padding-right: 0;
}
</style>
