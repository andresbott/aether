<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import PlayerControls from '@/components/layout/PlayerControls.vue'
import QueueSidebar from '@/components/layout/QueueSidebar.vue'
import ShortcutHelpOverlay from '@/components/layout/ShortcutHelpOverlay.vue'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'
import { useKeyboardShortcuts } from '@/composables/useKeyboardShortcuts'

const route = useRoute()
const scrollbarWidth = useScrollbarWidth()

// Bound here rather than in PlayerLayout: these are desktop-chrome actions,
// and the mobile shell (like the settings shell) deliberately gets none of
// them — the help overlay that teaches them never renders there either.
useKeyboardShortcuts()
</script>

<template>
    <div class="desktop-shell">
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
    </div>
</template>

<style>
.desktop-shell {
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
</style>
