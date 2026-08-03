<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import Toast from 'primevue/toast'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import PlayerControls from '@/components/layout/PlayerControls.vue'
import QueueSidebar from '@/components/layout/QueueSidebar.vue'
import ShortcutHelpOverlay from '@/components/layout/ShortcutHelpOverlay.vue'
import { useUiStore } from '@/store/uiStore'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'
import { useQueueSync } from '@/composables/useQueueSync'
import { useKeyboardShortcuts } from '@/composables/useKeyboardShortcuts'

const uiStore = useUiStore()
const route = useRoute()
const scrollbarWidth = useScrollbarWidth()
const queueSync = useQueueSync()

// Bound here rather than globally: these are player actions, and the settings
// shell is a separate layout that deliberately gets none of them.
useKeyboardShortcuts()

onMounted(async () => {
    uiStore.checkScreenWidth()
    window.addEventListener('resize', uiStore.checkScreenWidth)

    // Adopt the queue saved from another browser/device before arming the save
    // side: starting first would let a debounced local save race the state
    // arriving from the server.
    await queueSync.restore()
    queueSync.start()
})

onUnmounted(() => {
    window.removeEventListener('resize', uiStore.checkScreenWidth)
    queueSync.stop()
})
</script>

<template>
    <div class="app-container">
        <div class="body-row">
            <AppSidebar />

            <div class="content-area" :style="{ '--sb-w': scrollbarWidth + 'px' }">
                <main class="main-content" :class="{ 'main-content--flush': route.meta.flush }">
                    <RouterView />
                </main>
                <QueueSidebar v-if="route.name !== 'home'" />
            </div>
        </div>

        <PlayerControls />
        <!-- Last, so its badges measure a player bar that is already laid out. -->
        <ShortcutHelpOverlay />
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
