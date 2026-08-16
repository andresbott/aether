<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import Toast from 'primevue/toast'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import QueueSidebar from '@/components/layout/QueueSidebar.vue'
import DesktopShell from '@/layouts/DesktopShell.vue'
import MobileShell from '@/layouts/MobileShell.vue'
import { useQueueSync } from '@/composables/useQueueSync'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'
import { useViewport } from '@/composables/useViewport'
import { useMediaSession } from '@/composables/useMediaSession'

const route = useRoute()
const queueSync = useQueueSync()
// One chrome at a time: v-if (not CSS hiding) so there is never a duplicate
// tab order or ARIA tree, and shortcuts only bind in the desktop chrome. The
// route outlet lives HERE, outside the swap: a rotation replaces the chrome
// but must never unmount the active view — teardown bypasses the views'
// unsaved-edit guards (onBeforeRouteLeave / beforeunload), which are written
// for navigation, and would silently discard staged edits.
const { shell } = useViewport()
// Recipes read --sb-w in both shells; on phone overlay scrollbars it is 0.
const scrollbarWidth = useScrollbarWidth()

// Lock-screen / hardware-key controls; shell-independent, so it lives here
// rather than in either shell.
useMediaSession()

onMounted(async () => {
    // Adopt the queue saved from another browser/device before arming the save
    // side: starting first would let a debounced local save race the state
    // arriving from the server.
    await queueSync.restore()
    queueSync.start()
})

onUnmounted(() => {
    queueSync.stop()
})
</script>

<template>
    <div class="player-shell" :class="shell === 'desktop' ? 'desktop-shell' : 'mobile-shell'">
        <div class="body-row">
            <AppSidebar v-if="shell === 'desktop'" />

            <div class="content-area" :style="{ '--sb-w': scrollbarWidth + 'px' }">
                <main class="main-content" :class="{ 'main-content--flush': route.meta.flush }">
                    <RouterView />
                </main>
                <QueueSidebar v-if="shell === 'desktop' && route.name !== 'home'" />
            </div>
        </div>

        <DesktopShell v-if="shell === 'desktop'" />
        <MobileShell v-else />
    </div>
    <Toast />
</template>

<style>
.player-shell {
    display: flex;
    flex-direction: column;
    width: 100%;
    overflow: hidden;
    background-color: var(--app-background);
}

/* Both shells take their height from the app-shell chain in _main.scss (html is
   100dvh, body and #app are 100%) rather than measuring the viewport again
   here: a `100vh` column is the URL-bar-hidden height on mobile browsers and
   would hang below the visible area. */
.player-shell.desktop-shell {
    height: 100%;
    padding-bottom: var(--app-player-height);
}

.player-shell.mobile-shell {
    height: 100%;
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
    min-height: 0;
    overflow: hidden;
}
</style>
